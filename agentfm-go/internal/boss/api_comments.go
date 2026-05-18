package boss

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/peer"
)

// base64StdDecoder is captured at package scope so the inline
// base64Decode helper doesn't have to reference the encoding/base64
// package by name on every call.
var base64StdDecoder = base64.StdEncoding

// CommentSubmitRequest is the body for POST /v1/peers/{id}/comments.
// The caller signs a canonical byte sequence with their libp2p
// private key (Ed25519) and ships the signature alongside the body.
//
// For v1.3, ONLY self-submission is supported: rater_peer_id must
// match the host whose Boss is receiving the request (the operator's
// own libp2p identity). External-submitter delegation lands in v1.4
// after the protocol for verifying delegated authority is defined.
type CommentSubmitRequest struct {
	RaterPeerID         string `json:"rater_peer_id"`
	Text                string `json:"text"`
	Language            string `json:"language"`
	AttachedRatingHash  string `json:"attached_rating_hash,omitempty"` // hex
	SignatureBase64     string `json:"signature"`                       // base64-std
}

// CommentSubmitResponse is the JSON returned on 201 Created.
type CommentSubmitResponse struct {
	CID         string `json:"cid"`
	LedgerHash  string `json:"ledger_hash"`
}

// CommentSubmissionHandler is the public-facing handler for P4-3. It
// wires comments.Store + ledger.Append together. The Boss
// constructor calls Use to install the handler so the
// api_reputation.go umbrella router dispatches correctly.
type CommentSubmissionHandler struct {
	store    *comments.Store
	host     peerHostShim
	subjectFromPath func(path string) string
	signSubmission func(req *CommentSubmitRequest, signedBytes []byte) error // unused; placeholder for future delegated-submitter flow
}

// peerHostShim is the minimal interface CommentSubmissionHandler
// needs from a libp2p host (just the local PeerID). Decoupled so
// tests don't need a full libp2p stack.
type peerHostShim interface {
	ID() peer.ID
}

// NewCommentSubmissionHandler wires the dependencies. signer is
// reserved for future use (delegated submission); pass nil today.
func NewCommentSubmissionHandler(store *comments.Store, host peerHostShim) *CommentSubmissionHandler {
	return &CommentSubmissionHandler{store: store, host: host, subjectFromPath: defaultSubjectFromPath}
}

func defaultSubjectFromPath(path string) string { return extractPeerID(path, "/comments") }

// HandleHTTP services POST /v1/peers/{subject}/comments. The submitter
// signs the canonical bytes of the Comment envelope (everything
// except the Signature field) with their libp2p private key. We
// verify the signature against the embedded RaterPeerID before
// storing the body and appending to the ledger.
func (h *CommentSubmissionHandler) HandleHTTP(b *Boss, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only POST supported")
		return
	}
	if b.ledger == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "ledger_unavailable", "ledger not wired on this boss")
		return
	}
	subjectStr := h.subjectFromPath(r.URL.Path)
	if subjectStr == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing subject peer_id in path")
		return
	}
	subjectID, err := peer.Decode(subjectStr)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_peer_id", err.Error())
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024)) // request body cap — comments are <= 10 KiB
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "read_body", err.Error())
		return
	}
	var req CommentSubmitRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_json", err.Error())
		return
	}
	if req.RaterPeerID == "" || req.Text == "" || req.SignatureBase64 == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "missing_field", "rater_peer_id, text, and signature are required")
		return
	}
	if len(req.Text) > comments.MaxBodyBytes {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "body_too_large",
			fmt.Sprintf("text exceeds %d bytes", comments.MaxBodyBytes))
		return
	}
	raterID, err := peer.Decode(req.RaterPeerID)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_peer_id", err.Error())
		return
	}
	// v1.3: only self-submission. The rater MUST be this host's
	// own identity. Boss verifies by comparing PeerIDs.
	if h.host != nil && raterID != h.host.ID() {
		writeOpenAIError(w, http.StatusForbidden, errTypeInvalidRequest, "non_self_submitter",
			"v1.3 only supports self-submitted comments; rater_peer_id must match this boss's libp2p identity")
		return
	}

	// Store body, get CID.
	cid, err := h.store.Put([]byte(req.Text))
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "store_body", err.Error())
		return
	}

	// Build the Comment envelope. The ledger.Append path signs the
	// envelope under our own libp2p key (which IS the rater for
	// self-submission), so the caller's `signature` field is
	// effectively informational in v1.3 — but we still validate it
	// to lock the API shape for v1.4's delegated flow.
	now := time.Now().UnixNano()
	commentPB := &pb.Comment{
		RaterPeerId:     []byte(raterID),
		SubjectPeerId:   []byte(subjectID),
		TextCid:         cid,
		Language:        req.Language,
		TimestampUnixNs: now,
	}
	if req.AttachedRatingHash != "" {
		attached, err := hex.DecodeString(req.AttachedRatingHash)
		if err == nil && len(attached) == 32 {
			commentPB.AttachedRating = attached
		}
	}
	// Optional caller-supplied sig verification: caller hashes the
	// canonical Comment bytes (everything except Signature +
	// prev_hash, which the ledger fills) and signs the resulting
	// digest. We re-derive the same digest and verify against
	// rater's libp2p key. Mismatch = 401.
	if err := verifyCallerCommentSig(commentPB, raterID, req.SignatureBase64); err != nil {
		writeOpenAIError(w, http.StatusUnauthorized, errTypeInvalidRequest, "bad_signature", err.Error())
		return
	}

	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: commentPB}}
	hash, err := b.ledger.Append(r.Context(), entry)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "append_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, CommentSubmitResponse{
		CID:        comments.CIDString(cid),
		LedgerHash: hex.EncodeToString(hash[:]),
	})
}

// verifyCallerCommentSig validates that the caller's base64
// signature is a valid Ed25519 signature by raterID over
// SHA-256(CanonicalComment(comment-without-signature-or-prevhash)).
//
// For v1.3 self-submission, this is mostly a sanity check that the
// caller controls the rater key — the LEDGER's own signing pass
// produces the authoritative signature attached to the persisted
// entry.
func verifyCallerCommentSig(c *pb.Comment, raterID peer.ID, sigB64 string) error {
	if c == nil {
		return fmt.Errorf("nil comment")
	}
	canonical, err := pb.CanonicalComment(c)
	if err != nil {
		return fmt.Errorf("canonical comment: %w", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := base64Decode(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	pub, err := raterID.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extract pubkey: %w", err)
	}
	ok, err := pub.Verify(digest[:], sig)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// base64Decode wraps encoding/base64.StdEncoding.DecodeString to
// keep the import surface minimal at the top of this file.
func base64Decode(s string) ([]byte, error) {
	return base64StdDecoder.DecodeString(s)
}

// handleCommentBodyGet services GET /v1/peers/{id}/comments/{cid}.
//
// The CID is hex-encoded (same format as CommentSubmitResponse.CID and the
// on-disk path). The response body is the raw comment text with content-type
// text/plain; charset=utf-8.
//
// Errors:
//   - 400: CID is malformed hex
//   - 404: CID not found in local body store
//   - 503: commentsStore not wired on this boss
func (b *Boss) handleCommentBodyGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only GET supported")
		return
	}
	if b.commentsStore == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "comments_unavailable", "comments store not wired on this boss")
		return
	}

	// Extract the hex CID from the path: /v1/peers/{id}/comments/{cid}
	cidHex := extractCommentCID(r.URL.Path)
	if cidHex == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing CID in path")
		return
	}
	cidBytes, err := hex.DecodeString(cidHex)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_cid", "CID is not valid hex: "+err.Error())
		return
	}

	body, err := b.commentsStore.Get(cidBytes)
	if err != nil {
		if errors.Is(err, comments.ErrNotFound) || errors.Is(err, comments.ErrCIDMismatch) {
			writeOpenAIError(w, http.StatusNotFound, errTypeInvalidRequest, "comment_not_found", "comment body not found for this CID")
			return
		}
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "store_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// extractCommentCID extracts the hex CID from a URL path of the form
// /v1/peers/{id}/comments/{cid}. Returns "" on malformed input.
func extractCommentCID(urlPath string) string {
	// Find the "/comments/" separator.
	const sep = "/comments/"
	idx := strings.Index(urlPath, sep)
	if idx < 0 {
		return ""
	}
	cid := urlPath[idx+len(sep):]
	// Reject if there are further path segments.
	if strings.Contains(cid, "/") {
		return ""
	}
	return cid
}

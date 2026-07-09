package boss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// commentTestLedger is a stubLedger that records appended entries
// so tests can verify them.
type commentTestLedger struct {
	stubLedger
	appended []*pb.SignedEntry
	lastHash [32]byte
}

func (c *commentTestLedger) Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	c.appended = append(c.appended, payload)
	c.lastHash = sha256.Sum256([]byte{byte(len(c.appended))})
	return c.lastHash, nil
}

// hostStub implements peerHostShim for tests.
type hostStub struct{ id peer.ID }

func (h hostStub) ID() peer.ID { return h.id }

// commentTestRig packages everything an HTTP submission test needs.
type commentTestRig struct {
	boss    *Boss
	store   *comments.Store
	rater   peer.ID
	priv    crypto.PrivKey
	subject peer.ID
	ledger  *commentTestLedger
}

func newCommentRig(t *testing.T) *commentTestRig {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen rater key: %v", err)
	}
	rater, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("rater id: %v", err)
	}
	_, subjPub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen subject key: %v", err)
	}
	subject, _ := peer.IDFromPublicKey(subjPub)

	store, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}
	led := &commentTestLedger{}
	host := hostStub{id: rater} // rater IS this boss's identity (self-submission)

	handler := NewCommentSubmissionHandler(store, host)
	b := &Boss{
		ledger:                   led,
		commentSubmissionHandler: func(w http.ResponseWriter, r *http.Request) { handler.HandleHTTP(nil, w, r) },
	}
	// We need a closure that captures b — re-wire after construction
	// so handler.HandleHTTP sees the right Boss.
	b.commentSubmissionHandler = func(w http.ResponseWriter, r *http.Request) { handler.HandleHTTP(b, w, r) }

	return &commentTestRig{
		boss:    b,
		store:   store,
		rater:   rater,
		priv:    priv,
		subject: subject,
		ledger:  led,
	}
}

// signSubmission constructs the canonical bytes the API expects and
// returns a base64 signature.
func signSubmission(t *testing.T, priv crypto.PrivKey, raterID, subjectID peer.ID, text, language string, timestampNs int64) string {
	t.Helper()
	c := &pb.Comment{
		RaterPeerId:     []byte(raterID),
		SubjectPeerId:   []byte(subjectID),
		TextCid:         comments.CIDOf([]byte(text)),
		Language:        language,
		TimestampUnixNs: timestampNs,
	}
	canonical, err := pb.CanonicalComment(c)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := priv.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// TestCommentSubmission_HappyPath verifies a correctly-signed self-submission
// is accepted (201), the ledger is appended once, and the appended comment
// carries the caller-supplied timestamp. The handler takes the timestamp from
// the signed request body, so the caller's signature verifies deterministically.
func TestCommentSubmission_HappyPath(t *testing.T) {
	rig := newCommentRig(t)
	const ts = int64(1717171717000000000)
	sig := signSubmission(t, rig.priv, rig.rater, rig.subject, "great agent", "en", ts)
	body, _ := json.Marshal(CommentSubmitRequest{
		RaterPeerID:     rig.rater.String(),
		Text:            "great agent",
		Language:        "en",
		TimestampUnixNs: ts,
		SignatureBase64: sig,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/"+rig.subject.String()+"/comments",
		bytes.NewReader(body))
	rig.boss.handlePeers(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if len(rig.ledger.appended) != 1 {
		t.Fatalf("appended %d entries, want 1", len(rig.ledger.appended))
	}
	got := rig.ledger.appended[0].GetComment()
	if got == nil {
		t.Fatal("appended entry is not a Comment")
	}
	if got.TimestampUnixNs != ts {
		t.Errorf("ledger comment timestamp = %d, want %d (from request)", got.TimestampUnixNs, ts)
	}
}

func TestCommentSubmission_BadJSON(t *testing.T) {
	rig := newCommentRig(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/"+rig.subject.String()+"/comments",
		strings.NewReader("not json"))
	rig.boss.handlePeers(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCommentSubmission_MissingFields(t *testing.T) {
	rig := newCommentRig(t)
	body, _ := json.Marshal(map[string]string{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/"+rig.subject.String()+"/comments",
		bytes.NewReader(body))
	rig.boss.handlePeers(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCommentSubmission_NonSelfSubmitter_Forbidden(t *testing.T) {
	rig := newCommentRig(t)
	// Mint a different rater.
	otherPriv, otherPub, _ := crypto.GenerateEd25519Key(nil)
	otherRater, _ := peer.IDFromPublicKey(otherPub)

	sig := signSubmission(t, otherPriv, otherRater, rig.subject, "hello", "en", 1)
	body, _ := json.Marshal(CommentSubmitRequest{
		RaterPeerID:     otherRater.String(),
		Text:            "hello",
		Language:        "en",
		TimestampUnixNs: 1,
		SignatureBase64: sig,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/"+rig.subject.String()+"/comments",
		bytes.NewReader(body))
	rig.boss.handlePeers(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCommentSubmission_BodyTooLarge_Rejected(t *testing.T) {
	rig := newCommentRig(t)
	big := strings.Repeat("a", comments.MaxBodyBytes+10)
	sig := signSubmission(t, rig.priv, rig.rater, rig.subject, big, "en", 1)
	body, _ := json.Marshal(CommentSubmitRequest{
		RaterPeerID:     rig.rater.String(),
		Text:            big,
		Language:        "en",
		TimestampUnixNs: 1,
		SignatureBase64: sig,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/"+rig.subject.String()+"/comments",
		bytes.NewReader(body))
	rig.boss.handlePeers(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body too large)", rec.Code)
	}
}

// Compile-time check the stubLedger satisfies the interface.
var _ ledger.Ledger = (*commentTestLedger)(nil)

// io.Reader unused-import guard for some test variants.
var _ io.Reader = (*bytes.Reader)(nil)

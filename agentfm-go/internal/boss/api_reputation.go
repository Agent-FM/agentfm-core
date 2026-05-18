// P4-2: HTTP endpoints exposing ledger state.
//
//	GET /v1/peers/{peer_id}/reputation
//	GET /v1/peers/{peer_id}/log?from=N&limit=M
//	GET /v1/peers/{peer_id}/proof?entry={hex_hash}
//
// All endpoints respect the existing bearer-token auth + rate-limit
// middleware (same wrapper pattern as /v1/chat/completions). Error
// envelopes match the OpenAI-compatible shape.
//
// Path parsing is hand-rolled (no router dep) — net/http's pattern
// support in Go 1.22+ would do this neatly but the existing codebase
// uses ServeMux without patterns, so we keep that consistency.
package boss

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/reputation"

	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// reputationResponse is the JSON returned by GET /v1/peers/{id}/reputation.
type reputationResponse struct {
	PeerID         string             `json:"peer_id"`
	Scores         map[string]float64 `json:"scores"`
	RatingCount    int                `json:"rating_count"`
	LastUpdated    string             `json:"last_updated,omitempty"`
	IsEquivocator  bool               `json:"is_equivocator"`
	AgentImageRef  string             `json:"agent_image_ref,omitempty"`
	AgentImageDgst string             `json:"agent_image_digest,omitempty"`
	AgentCapab     string             `json:"agent_capability,omitempty"`
}

type logResponse struct {
	Entries []logEntryDTO `json:"entries"`
	Head    *headDTO      `json:"head,omitempty"`
}

type logEntryDTO struct {
	Idx        uint64  `json:"idx"`
	Hash       string  `json:"hash"`
	PrevHash   string  `json:"prev_hash"`
	Kind       string  `json:"kind"`
	Score      float64 `json:"score,omitempty"`
	Dimension  string  `json:"dimension,omitempty"`
	Context    string  `json:"context,omitempty"`
	Rater      string  `json:"rater,omitempty"`
	Subject    string  `json:"subject,omitempty"`
	ReceivedAt string  `json:"received_at,omitempty"`
}

type headDTO struct {
	TreeSize     uint64 `json:"tree_size"`
	RootHash     string `json:"root_hash"`
	WitnessCount int    `json:"witness_count"`
	SignedAt     string `json:"signed_at,omitempty"`
}

type proofResponse struct {
	EntryHash string   `json:"entry_hash"`
	Position  uint64   `json:"position"`
	AuditPath []string `json:"audit_path"`
	Head      headDTO  `json:"head"`
}

// handlePeers is the umbrella handler for /v1/peers/* routes. It
// branches on the suffix (.../reputation, .../log, .../proof, .../comments,
// or bare {id}) so we register a single ServeMux entry — saves wrestling with
// Go-1.22-style path patterns when the existing codebase doesn't use any.
func (b *Boss) handlePeers(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/reputation"):
		b.handleReputation(w, r)
	case strings.HasSuffix(path, "/log"):
		b.handleLog(w, r)
	case strings.HasSuffix(path, "/proof"):
		b.handleProof(w, r)
	case strings.HasSuffix(path, "/comments"):
		// P4-3 — POST /v1/peers/{id}/comments submits a comment.
		b.handleCommentSubmission(w, r)
	case strings.Contains(path, "/comments/"):
		// GET /v1/peers/{id}/comments/{cid} hydrates a comment body.
		b.handleCommentBodyGet(w, r)
	case isPeerSummaryPath(path):
		// /v1/peers/{id} with no trailing sub-resource.
		b.handlePeerGet(w, r)
	default:
		writeOpenAIError(w, http.StatusNotFound, errTypeInvalidRequest, "not_found", "unknown peers sub-resource")
	}
}

// isPeerSummaryPath returns true when the URL path is exactly
// /v1/peers/{id} (one path segment after the prefix, no sub-resource).
func isPeerSummaryPath(urlPath string) bool {
	const prefix = "/v1/peers/"
	if !strings.HasPrefix(urlPath, prefix) {
		return false
	}
	rest := urlPath[len(prefix):]
	// No slash in the remainder means it's a bare peer ID.
	return rest != "" && !strings.Contains(rest, "/")
}

// handleReputation services GET /v1/peers/{id}/reputation.
func (b *Boss) handleReputation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only GET supported")
		return
	}
	if b.ledger == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "ledger_unavailable", "ledger not wired on this boss")
		return
	}
	peerIDStr := extractPeerID(r.URL.Path, "/reputation")
	if peerIDStr == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing peer_id in path")
		return
	}
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_peer_id", err.Error())
		return
	}

	ctx := r.Context()
	// Fresh-on-read recompute: small-mesh deployments want the
	// score reflected in /v1/peers/.../reputation immediately
	// after a rating fires, not 60s later when the ticker runs.
	// On a ~1k-entry ledger this takes ~5ms; production deploys
	// with very large ledgers can skip this by unsetting ReadStore.
	if b.reputationEngine != nil && b.readStore != nil {
		_, _ = b.reputationEngine.Recompute(ctx, b.readStore)
	}
	view, err := buildReputationView(ctx, b.ledger, b.reputationEngine, []byte(pid), peerIDStr)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "ledger_error", err.Error())
		return
	}
	// Populate rating_count + last_updated by scanning own + inbox.
	// Cheap on small-mesh deploys; production deployments with
	// massive ledgers can drop this scan by setting ReadStore=nil.
	if b.readStore != nil {
		count, latest := countSubjectRatings(ctx, b.readStore, []byte(pid))
		view.RatingCount = count
		if !latest.IsZero() {
			view.LastUpdated = latest.UTC().Format(time.RFC3339)
		}
	}

	// Decorate with the live telemetry profile if we have one — saves
	// the caller a round trip to /api/workers.
	b.mu.RLock()
	if profile, ok := b.activeWorkers[peerIDStr]; ok {
		view.AgentImageRef = profile.AgentImageRef
		view.AgentImageDgst = profile.AgentImageDigest
		view.AgentCapab = profile.AgentCapability
	}
	b.mu.RUnlock()

	writeJSON(w, http.StatusOK, view)
}

// handleLog services GET /v1/peers/{id}/log?limit=N&offset=M.
//
// Returns a paginated list of PeerEntries for the requested subject peer,
// gathered from both the boss's own log and inbox. Each entry is decorated
// with rater_status ("verified" if rater honesty >= 0.1, else "unverified")
// and rater_honesty_score from the live reputation engine.
func (b *Boss) handleLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only GET supported")
		return
	}
	if b.readStore == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "ledger_unavailable", "ledger not wired on this boss")
		return
	}

	peerIDStr := extractPeerID(r.URL.Path, "/log")
	if peerIDStr == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing peer_id in path")
		return
	}
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_peer_id", err.Error())
		return
	}

	limit := int(parseUintQuery(r, "limit", 50))
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := int(parseUintQuery(r, "offset", 0))

	ctx := r.Context()

	// Gather all matching entries (up to limit+offset so we can slice).
	all, err := GatherPeerEntries(ctx, b.readStore, pid, limit+offset)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "gather_error", err.Error())
		return
	}

	// Apply offset.
	if offset > len(all) {
		offset = len(all)
	}
	page := all[offset:]
	if len(page) > limit {
		page = page[:limit]
	}

	// Decorate entries with rater trust info.
	for i := range page {
		raterStr := page[i].Rater.String()
		var honestyScore float64
		if b.reputationEngine != nil {
			honestyScore = b.reputationEngine.Score(raterStr)
		}
		page[i].RaterHonestyScore = honestyScore
		if honestyScore >= 0.1 {
			page[i].RaterStatus = "verified"
		} else {
			page[i].RaterStatus = "unverified"
		}
	}

	type peerLogResponse struct {
		Subject  string      `json:"subject"`
		Limit    int         `json:"limit"`
		Offset   int         `json:"offset"`
		Returned int         `json:"returned"`
		Entries  []PeerEntry `json:"entries"`
	}
	writeJSON(w, http.StatusOK, peerLogResponse{
		Subject:  peerIDStr,
		Limit:    limit,
		Offset:   offset,
		Returned: len(page),
		Entries:  page,
	})
}

// handleProof services GET /v1/peers/{id}/proof?entry={hex}.
func (b *Boss) handleProof(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only GET supported")
		return
	}
	if b.ledger == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "ledger_unavailable", "ledger not wired on this boss")
		return
	}
	hashHex := r.URL.Query().Get("entry")
	if hashHex == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing entry query parameter")
		return
	}
	bs, err := hex.DecodeString(hashHex)
	if err != nil || len(bs) != 32 {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "entry must be a 64-char hex string")
		return
	}
	var hash [32]byte
	copy(hash[:], bs)

	ctx := r.Context()
	proof, err := b.ledger.Prove(ctx, hash)
	if err != nil {
		if errors.Is(err, ledger.ErrEntryNotInLog) {
			writeOpenAIError(w, http.StatusNotFound, errTypeInvalidRequest, "entry_not_found", err.Error())
			return
		}
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "prove_error", err.Error())
		return
	}

	audit := make([]string, len(proof.AuditPath))
	for i, p := range proof.AuditPath {
		audit[i] = hex.EncodeToString(p)
	}
	resp := proofResponse{
		EntryHash: hashHex,
		Position:  proof.Position,
		AuditPath: audit,
		Head: headDTO{
			TreeSize:     proof.LogHead.TreeSize,
			RootHash:     hex.EncodeToString(proof.LogHead.RootHash),
			WitnessCount: len(proof.LogHead.WitnessSigs),
			SignedAt:     time.Unix(0, proof.LogHead.TimestampUnixNs).UTC().Format(time.RFC3339),
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// buildReputationView assembles the response for
// /v1/peers/{id}/reputation. Pulls IsEquivocator from the ledger
// (permanent floor at -1.0) and the live honesty score from the
// reputation engine when one is wired.
//
// Without an engine (e.g. unit-test boss), the response carries
// only the equivocator floor — the same behaviour as P1-6's CLI
// when scoring isn't available.
func buildReputationView(ctx context.Context, l ledger.Ledger, eng reputationEngineIface, subject []byte, subjectStr string) (*reputationResponse, error) {
	view := &reputationResponse{
		PeerID: subjectStr,
		Scores: make(map[string]float64),
	}
	marked, err := l.IsEquivocator(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("is equivocator: %w", err)
	}
	view.IsEquivocator = marked
	if marked {
		// Equivocator floor overrides everything.
		view.Scores["honesty"] = reputation.EquivocatorFloor
		return view, nil
	}
	if eng != nil {
		score := eng.Score(subjectStr)
		view.Scores["honesty"] = score
	}
	// rating_count / last_updated remain zero until a future ticket
	// wires the boss to count entries about subject (today the engine
	// is the source of truth for "what we think").
	return view, nil
}

// writeJSON marshals v as JSON and writes it with status. Used by
// every endpoint here for symmetry.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// extractPeerID parses `{peer_id}` out of `/v1/peers/{peer_id}{suffix}`.
// Returns "" on malformed input.
func extractPeerID(urlPath, suffix string) string {
	const prefix = "/v1/peers/"
	if !strings.HasPrefix(urlPath, prefix) {
		return ""
	}
	rest := urlPath[len(prefix):]
	if suffix != "" {
		idx := strings.Index(rest, suffix)
		if idx <= 0 {
			return ""
		}
		return rest[:idx]
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

func parseUintQuery(r *http.Request, key string, def uint64) uint64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// countSubjectRatings scans own log + inbox for Rating entries about
// subject and returns (count, latest timestamp). Used by the
// reputation HTTP handler so the response carries rating_count +
// last_updated alongside the score. O(n) over total ratings; cheap
// on small ledgers, but the caller is expected to skip this on
// massive deployments by not wiring ReadStore.
func countSubjectRatings(ctx context.Context, s *store.Store, subject []byte) (int, time.Time) {
	count := 0
	var latest time.Time
	check := func(payload []byte, ts int64) {
		var signed pb.SignedEntry
		if err := proto.Unmarshal(payload, &signed); err != nil {
			return
		}
		r := signed.GetRating()
		if r == nil {
			return
		}
		if !bytesEqualPB(r.SubjectPeerId, subject) {
			return
		}
		count++
		t := time.Unix(0, ts)
		if t.After(latest) {
			latest = t
		}
	}
	_ = s.IterateAllOwnEntries(ctx, func(e *store.Entry) error {
		check(e.Payload, e.InsertedAt)
		return nil
	})
	_ = s.IterateAllInboxEntries(ctx, func(e *store.InboxEntry) error {
		check(e.Payload, e.ReceivedAt)
		return nil
	})
	return count, latest
}

// peerSummaryResponse is the JSON body for GET /v1/peers/{id}.
type peerSummaryResponse struct {
	PeerID               string       `json:"peer_id"`
	AgentName            string       `json:"agent_name"`
	Online               bool         `json:"online"`
	LastSeen             *time.Time   `json:"last_seen,omitempty"`
	HonestyScore         float64      `json:"honesty_score"`
	IsEquivocator        bool         `json:"is_equivocator"`
	DispatchAllowed      bool         `json:"dispatch_allowed"`
	DispatchRefuseReason string       `json:"dispatch_refuse_reason,omitempty"`
	EntriesCount         int          `json:"entries_count"`
	LastEntryAt          *time.Time   `json:"last_entry_at,omitempty"`
	AdvertisedImageRef   string       `json:"advertised_image_ref,omitempty"`
	AdvertisedImageDgst  string       `json:"advertised_image_digest,omitempty"`
	AdvertisedCapability string       `json:"advertised_capability,omitempty"`
	RaterSummary         raterSummary `json:"rater_summary"`
}

type raterSummary struct {
	VerifiedRatersCount   int `json:"verified_raters_count"`
	UnverifiedRatersCount int `json:"unverified_raters_count"`
}

// handlePeerGet services GET /v1/peers/{id}.
func (b *Boss) handlePeerGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, "method_not_allowed", "only GET supported")
		return
	}
	if b.readStore == nil {
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, "ledger_unavailable", "ledger not wired on this boss")
		return
	}

	peerIDStr := extractPeerID(r.URL.Path, "")
	if peerIDStr == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_request", "missing peer_id in path")
		return
	}
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, "bad_peer_id", err.Error())
		return
	}

	ctx := r.Context()

	// Gather all entries for this peer (no cap — we need the full count).
	entries, err := GatherPeerEntries(ctx, b.readStore, pid, 0)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, "gather_error", err.Error())
		return
	}

	// Compute rater summary: distinct raters, verified vs. unverified.
	seenRaters := make(map[string]struct{})
	verifiedCount := 0
	unverifiedCount := 0
	var lastEntryAt *time.Time
	for _, e := range entries {
		raterStr := e.Rater.String()
		if _, seen := seenRaters[raterStr]; !seen {
			seenRaters[raterStr] = struct{}{}
			var hs float64
			if b.reputationEngine != nil {
				hs = b.reputationEngine.Score(raterStr)
			}
			if hs >= 0.1 {
				verifiedCount++
			} else {
				unverifiedCount++
			}
		}
		if lastEntryAt == nil || e.ReceivedAt.After(*lastEntryAt) {
			t := e.ReceivedAt
			lastEntryAt = &t
		}
	}

	honesty, equivocator, dispatchAllowed, refuseReason := b.computeTrustView(peerIDStr)

	resp := peerSummaryResponse{
		PeerID:               peerIDStr,
		HonestyScore:         honesty,
		IsEquivocator:        equivocator,
		DispatchAllowed:      dispatchAllowed,
		DispatchRefuseReason: refuseReason,
		EntriesCount:         len(entries),
		LastEntryAt:          lastEntryAt,
		RaterSummary: raterSummary{
			VerifiedRatersCount:   verifiedCount,
			UnverifiedRatersCount: unverifiedCount,
		},
	}

	// Populate live telemetry fields if the peer is in activeWorkers.
	b.mu.RLock()
	if profile, ok := b.activeWorkers[peerIDStr]; ok {
		resp.Online = true
		resp.AgentName = profile.AgentName
		resp.AdvertisedImageRef = profile.AgentImageRef
		resp.AdvertisedImageDgst = profile.AgentImageDigest
		resp.AdvertisedCapability = profile.AgentCapability
		if seen, ok := b.lastSeen[peerIDStr]; ok {
			resp.LastSeen = &seen
		}
	}
	b.mu.RUnlock()

	writeJSON(w, http.StatusOK, resp)
}

// handleCommentSubmission is filled in by api_comments.go (P4-3).
// Declared here so the umbrella router compiles in P4-2 alone — the
// stub returns 501 until P4-3 replaces it.
func (b *Boss) handleCommentSubmission(w http.ResponseWriter, r *http.Request) {
	// Defined in api_comments.go once P4-3 lands.
	if b.commentSubmissionHandler != nil {
		b.commentSubmissionHandler(w, r)
		return
	}
	writeOpenAIError(w, http.StatusNotImplemented, errTypeServerError, "not_implemented", "comment submission lands in P4-3")
}

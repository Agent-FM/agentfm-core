package boss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/reputation"
	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// openFreshPV opens a fresh store in a temp dir for peer_view tests.
func openFreshPV(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "pv.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newPeerIDPV(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, _ := peer.IDFromPublicKey(pub)
	return id
}

var pvInsertSeq int64

func insertInboxRating(t *testing.T, s *store.Store, rater, subject peer.ID, score float64) {
	t.Helper()
	pvInsertSeq++
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		TimestampUnixNs: time.Now().UnixNano() + pvInsertSeq, // unique ns to avoid hash collision
		PrevHash:        make([]byte, 32),
	}}}
	payload, _ := proto.Marshal(entry)
	var hash [32]byte
	copy(hash[:], payload)
	// Ensure distinct hash by varying multiple bytes (to avoid cycling on > 256 insertions)
	hash[30] ^= byte(pvInsertSeq >> 8)
	hash[31] ^= byte(pvInsertSeq)
	if err := s.InsertInboxEntry(context.Background(), []byte(rater), hash, [32]byte{}, payload); err != nil {
		t.Fatalf("InsertInboxEntry: %v", err)
	}
}

func insertOwnRating(t *testing.T, s *store.Store, rater, subject peer.ID, score float64) {
	t.Helper()
	pvInsertSeq++
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		TimestampUnixNs: time.Now().UnixNano() + pvInsertSeq,
		PrevHash:        make([]byte, 32),
	}}}
	payload, _ := proto.Marshal(entry)
	var hash, prev [32]byte
	copy(hash[:], payload)
	// Ensure distinct hash by varying multiple bytes (to avoid cycling on > 256 insertions)
	hash[30] ^= byte(pvInsertSeq >> 8)
	hash[31] ^= byte(pvInsertSeq)
	_, err := s.AppendEntry(context.Background(), hash, prev, store.KindRating, payload, []byte{})
	if err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
}

// TestGatherPeerEntries_BothOwnAndInbox verifies that GatherPeerEntries
// collects entries from BOTH the own log and inbox for the requested subject.
func TestGatherPeerEntries_BothOwnAndInbox(t *testing.T) {
	s := openFreshPV(t)
	rater1 := newPeerIDPV(t)
	rater2 := newPeerIDPV(t)
	subject := newPeerIDPV(t)
	other := newPeerIDPV(t)

	// One own-log entry for subject, one inbox entry for subject,
	// one inbox entry for a different peer (should be filtered).
	insertOwnRating(t, s, rater1, subject, 0.8)
	insertInboxRating(t, s, rater2, subject, -0.4)
	insertInboxRating(t, s, rater1, other, 0.5) // different subject — must be excluded

	entries, err := GatherPeerEntries(context.Background(), s, subject, 50)
	if err != nil {
		t.Fatalf("GatherPeerEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for subject; got %d", len(entries))
	}
}

// TestGatherPeerEntries_LimitRespected verifies the limit cap.
func TestGatherPeerEntries_LimitRespected(t *testing.T) {
	s := openFreshPV(t)
	rater := newPeerIDPV(t)
	subject := newPeerIDPV(t)

	for i := 0; i < 5; i++ {
		insertInboxRating(t, s, rater, subject, 0.5)
	}

	entries, err := GatherPeerEntries(context.Background(), s, subject, 3)
	if err != nil {
		t.Fatalf("GatherPeerEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (limit); got %d", len(entries))
	}
}

// TestHandlePeerLog_RaterStatus verifies the paginated /v1/peers/{id}/log
// endpoint returns rater_status field and pagination metadata.
func TestHandlePeerLog_RaterStatus(t *testing.T) {
	s := openFreshPV(t)
	rater := newPeerIDPV(t)
	subject := newPeerIDPV(t)

	insertInboxRating(t, s, rater, subject, 0.8)

	// Seed the rater with a positive score so rater_status = "verified".
	seeds := []reputation.Seed{{PeerID: rater.String(), Score: 0.5}}
	eng := reputation.New(seeds, reputation.Config{})
	_, _ = eng.Recompute(context.Background(), s)

	b := &Boss{
		ledger:           &stubLedger{},
		readStore:        s,
		reputationEngine: eng,
		activeWorkers:    make(map[string]types.WorkerProfile), // Ensure types is used
		lastSeen:         make(map[string]time.Time),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+subject.String()+"/log?limit=10&offset=0", nil)
	b.handlePeers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Check envelope fields.
	mustHaveKeys := []string{"subject", "returned", "limit", "offset", "entries"}
	for _, k := range mustHaveKeys {
		if _, ok := resp[k]; !ok {
			t.Errorf("key %q missing from response; got: %v", k, mapKeysAny(resp))
		}
	}

	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries is not an array: %T", resp["entries"])
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one entry")
	}

	entry, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("entry[0] is not a map")
	}
	if _, ok := entry["rater_status"]; !ok {
		t.Errorf("rater_status missing from entry; body=%s", rec.Body.String())
	}
	if _, ok := entry["rater_honesty_score"]; !ok {
		t.Errorf("rater_honesty_score missing from entry; body=%s", rec.Body.String())
	}
}

func mapKeysAny(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestHandlePeerLog_LimitCapAt500 verifies that ?limit=600 returns capped at 500, not reset to 50.
func TestHandlePeerLog_LimitCapAt500(t *testing.T) {
	s := openFreshPV(t)
	rater := newPeerIDPV(t)
	subject := newPeerIDPV(t)

	// Insert 520 entries — enough to test the 500 cap boundary.
	// The key behavior: limit=600 should cap to 500, not reset to 50.
	for i := 0; i < 520; i++ {
		insertInboxRating(t, s, rater, subject, 0.5)
	}

	// The HTTP handler will receive limit=600, cap it to 500, then call
	// GatherPeerEntries(ctx, store, pid, 500+0) = GatherPeerEntries(..., 500).
	// This should return up to 500 entries (we have 520).
	gathered, err := GatherPeerEntries(context.Background(), s, subject, 500)
	if err != nil {
		t.Fatalf("GatherPeerEntries: %v", err)
	}
	// GatherPeerEntries should cap at 500.
	if len(gathered) != 500 {
		t.Errorf("GatherPeerEntries(limit=500) expected 500 entries (capped); got %d", len(gathered))
	}

	b := &Boss{
		ledger:           &stubLedger{},
		readStore:        s,
		reputationEngine: nil,
		activeWorkers:    make(map[string]types.WorkerProfile),
		lastSeen:         make(map[string]time.Time),
	}

	// Request with limit=600 over HTTP, which should be capped to 500 by the handler.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+subject.String()+"/log?limit=600&offset=0", nil)
	b.handlePeers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries is not an array: %T", resp["entries"])
	}

	// The handler caps limit=600 to 500, then passes it to GatherPeerEntries.
	if len(entries) != 500 {
		t.Errorf("expected 500 entries (capped from limit=600); got %d", len(entries))
	}
	// Verify the response's reported limit field is also 500.
	if limit, ok := resp["limit"].(float64); !ok || int(limit) != 500 {
		t.Errorf("expected response limit=500; got %v", resp["limit"])
	}
}

// TestGatherPeerEntries_UncappedScan verifies that GatherPeerEntries with limit<=0
// returns ALL matching entries (uncapped), not defaulting to 50.
func TestGatherPeerEntries_UncappedScan(t *testing.T) {
	s := openFreshPV(t)
	rater := newPeerIDPV(t)
	subject := newPeerIDPV(t)

	// Insert 120 entries — more than the old hardcoded default of 50.
	for i := 0; i < 120; i++ {
		insertInboxRating(t, s, rater, subject, 0.5)
	}

	// Pass limit=0 (uncapped request, used by handlePeerGet).
	entries, err := GatherPeerEntries(context.Background(), s, subject, 0)
	if err != nil {
		t.Fatalf("GatherPeerEntries: %v", err)
	}

	// Must return all 120 entries, not silently cap to 50.
	if len(entries) != 120 {
		t.Errorf("expected 120 entries (uncapped, limit=0); got %d", len(entries))
	}
}

// Check we can import types without an explicit unused-import error
var _ = strings.Contains

// ---------------------------------------------------------------------------
// Phase 7: RenderPeerView tests
// ---------------------------------------------------------------------------

// TestRenderPeerView_RendersRatingScore verifies that a rating appended via
// testutil.AppendOwnRating appears in the rendered output with the score value.
func TestRenderPeerView_RendersRatingScore(t *testing.T) {
	b, store := newTestBossWithLedger(t)
	// Wire readStore so GatherPeerEntries finds entries.
	b.readStore = store

	subj := newPeerIDPV(t)
	insertOwnRating(t, store, b.node.Host.ID(), subj, -0.3)

	out := b.RenderPeerView(context.Background(), subj.String())
	if !strings.Contains(out, "-0.30") {
		t.Errorf("rendered view missing -0.30:\n%s", out)
	}
}

// TestRenderPeerView_TagsUnverifiedRaters verifies that a rater whose
// EigenTrust honesty score is < 0.1 is tagged as [unverified].
func TestRenderPeerView_TagsUnverifiedRaters(t *testing.T) {
	b, store := newTestBossWithLedger(t)
	b.readStore = store
	// No reputation engine → Score returns 0.0 < 0.1 → unverified.

	subj := newPeerIDPV(t)
	insertOwnRating(t, store, b.node.Host.ID(), subj, -0.3)

	out := b.RenderPeerView(context.Background(), subj.String())
	if !strings.Contains(out, "[unverified]") {
		t.Errorf("rendered view missing [unverified] tag:\n%s", out)
	}
}

// TestRenderPeerView_NoEntries verifies the empty-state message is shown when
// there are no ledger entries for the requested peer.
func TestRenderPeerView_NoEntries(t *testing.T) {
	b, store := newTestBossWithLedger(t)
	b.readStore = store

	subj := newPeerIDPV(t)
	out := b.RenderPeerView(context.Background(), subj.String())
	if !strings.Contains(out, "No ledger entries about this peer yet") {
		t.Errorf("expected empty-state message:\n%s", out)
	}
}

// TestRenderPeerView_HeaderContainsPeerID verifies the header line includes
// the short peer ID.
func TestRenderPeerView_HeaderContainsPeerID(t *testing.T) {
	b, store := newTestBossWithLedger(t)
	b.readStore = store

	subj := newPeerIDPV(t)
	out := b.RenderPeerView(context.Background(), subj.String())
	wantFragment := subj.String()[:12]
	if !strings.Contains(out, wantFragment) {
		t.Errorf("rendered view missing short peer ID %q:\n%s", wantFragment, out)
	}
}


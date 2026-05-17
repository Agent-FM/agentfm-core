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
	hash[31] ^= byte(pvInsertSeq) // ensure distinct hash
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
	mustHaveKeys := []string{"subject", "count", "limit", "offset", "entries"}
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

// Check we can import types without an explicit unused-import error
var _ = strings.Contains

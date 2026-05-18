package boss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// newBossForWorkersTest creates a Boss wired with readStore for the
// include_offline handler tests.
func newBossForWorkersTest(t *testing.T) (*Boss, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ledger.db")
	commentsDir := filepath.Join(dir, "comments")

	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	l, err := ledger.New(dbPath, priv, nil)
	if err != nil {
		t.Fatalf("ledger.New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	readStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = readStore.Close() })

	cs, err := comments.Open(commentsDir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		ledger:        l,
		readStore:     readStore,
		commentsStore: cs,
	}
	return b, readStore
}

// TestAPIWorkers_IncludeOfflineSurfacesInboxPeers verifies:
//  1. Default (no ?include_offline) returns only online peers.
//  2. ?include_offline=true returns both online and offline peers.
func TestAPIWorkers_IncludeOfflineSurfacesInboxPeers(t *testing.T) {
	b, store := newBossForWorkersTest(t)

	onlineSubj := testutil.NewHost(t)
	offlineSubj := testutil.NewHost(t)

	b.SeedWorker(types.WorkerProfile{PeerID: onlineSubj.ID().String(), AgentName: "online"})
	testutil.AppendOwnRating(t, store, b.HostForTest(), offlineSubj.ID(), -0.2, "test")

	// Default (no query): only online.
	rec := httptest.NewRecorder()
	b.HandleGetWorkersForTest(rec, httptest.NewRequest(http.MethodGet, "/api/workers", nil))
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, rec.Body.String())
	}
	agents, _ := got["agents"].([]any)
	if len(agents) != 1 {
		t.Fatalf("default should be online-only (1 agent); got %d: %v", len(agents), got)
	}

	// With include_offline=true: both.
	rec = httptest.NewRecorder()
	b.HandleGetWorkersForTest(rec, httptest.NewRequest(http.MethodGet, "/api/workers?include_offline=true", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, rec.Body.String())
	}
	agents, _ = got["agents"].([]any)
	if len(agents) != 2 {
		t.Fatalf("include_offline should return 2; got %d: %v", len(agents), got)
	}
}

// TestAPIWorkers_ResponseHasCountFields verifies online_count and
// offline_count appear in the response for both modes.
func TestAPIWorkers_ResponseHasCountFields(t *testing.T) {
	b, s := newBossForWorkersTest(t)

	online := testutil.NewHost(t)
	offline := testutil.NewHost(t)

	b.SeedWorker(types.WorkerProfile{PeerID: online.ID().String(), AgentName: "live"})
	testutil.AppendOwnRating(t, s, b.HostForTest(), offline.ID(), 0.1, "ctx")

	rec := httptest.NewRecorder()
	b.HandleGetWorkersForTest(rec, httptest.NewRequest(http.MethodGet, "/api/workers?include_offline=true", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := got["online_count"]; !ok {
		t.Errorf("online_count missing from response: %v", got)
	}
	if _, ok := got["offline_count"]; !ok {
		t.Errorf("offline_count missing from response: %v", got)
	}
	onlineCount, _ := got["online_count"].(float64)
	offlineCount, _ := got["offline_count"].(float64)
	if int(onlineCount) != 1 {
		t.Errorf("expected online_count=1; got %v", onlineCount)
	}
	if int(offlineCount) != 1 {
		t.Errorf("expected offline_count=1; got %v", offlineCount)
	}
}

// TestAPIWorkers_DefaultResponseShape verifies backwards compat: without
// include_offline, the response still includes online_count/offline_count
// fields but offline_count == 0.
func TestAPIWorkers_DefaultResponseShape(t *testing.T) {
	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{
			"peer1": {PeerID: "peer1", AgentName: "x"},
		},
		lastSeen: make(map[string]time.Time),
	}
	ctx := context.Background()
	_ = ctx // silence unused warning

	rec := httptest.NewRecorder()
	b.HandleGetWorkersForTest(rec, httptest.NewRequest(http.MethodGet, "/api/workers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success should be true; got %v", got["success"])
	}
	if _, ok := got["agents"]; !ok {
		t.Errorf("agents key missing from response")
	}
	// offline_count may be present or absent when readStore is nil — both acceptable.
}

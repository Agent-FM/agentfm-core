package boss

import (
	"context"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"path/filepath"
)

// newTestBossWithReadStore creates a Boss wired with a real ledger AND
// a readStore so ListKnownPeers can consult DistinctSubjects. Returns the
// boss and the readStore (which callers can also write to directly via the
// store API).
func newTestBossWithReadStore(t *testing.T) (*Boss, *store.Store) {
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
		t.Fatalf("store.Open (read handle): %v", err)
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

// TestListKnownPeers_IncludesOfflineFromInbox verifies that ListKnownPeers
// surfaces offline peers from the readStore in addition to online peers from
// activeWorkers.
func TestListKnownPeers_IncludesOfflineFromInbox(t *testing.T) {
	b, store := newTestBossWithReadStore(t)

	onlineSubj := testutil.NewHost(t)
	offlineSubj := testutil.NewHost(t)

	// Seed online peer.
	b.SeedWorker(types.WorkerProfile{
		PeerID: onlineSubj.ID().String(), AgentName: "online", CPUCores: 1,
	})

	// Seed offline peer via own-log entry.
	testutil.AppendOwnRating(t, store, b.HostForTest(), offlineSubj.ID(), -0.2, "test")

	known, err := b.ListKnownPeers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(known) < 2 {
		t.Fatalf("expected at least 2 (1 online + 1 offline); got %d", len(known))
	}

	var onlineFound, offlineFound bool
	for _, kp := range known {
		if kp.PeerID == onlineSubj.ID() && kp.IsOnline {
			onlineFound = true
		}
		if kp.PeerID == offlineSubj.ID() && !kp.IsOnline {
			offlineFound = true
		}
	}
	if !onlineFound {
		t.Error("online subject missing or wrong flag")
	}
	if !offlineFound {
		t.Error("offline subject missing or wrong flag")
	}
}

// TestListKnownPeers_OnlinePeersFirst verifies that online peers sort before
// offline ones.
func TestListKnownPeers_OnlinePeersFirst(t *testing.T) {
	b, s := newTestBossWithReadStore(t)

	online := testutil.NewHost(t)
	offline := testutil.NewHost(t)

	b.SeedWorker(types.WorkerProfile{PeerID: online.ID().String(), AgentName: "live"})
	testutil.AppendOwnRating(t, s, b.HostForTest(), offline.ID(), 0.1, "t")

	known, err := b.ListKnownPeers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(known) < 2 {
		t.Fatalf("expected >=2 peers; got %d", len(known))
	}
	// First entry must be online.
	if !known[0].IsOnline {
		t.Errorf("expected online peer to sort first; got IsOnline=%v", known[0].IsOnline)
	}
}

// TestListKnownPeers_NoStore verifies that ListKnownPeers still returns the
// online-only list when readStore is nil (no ledger wired).
func TestListKnownPeers_NoStore(t *testing.T) {
	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
	}
	online := testutil.NewHost(t)
	b.SeedWorker(types.WorkerProfile{PeerID: online.ID().String(), AgentName: "live"})

	known, err := b.ListKnownPeers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 {
		t.Fatalf("expected 1 online peer; got %d", len(known))
	}
}

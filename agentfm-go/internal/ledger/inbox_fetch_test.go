package ledger_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// TestInboxFetch_ServerStreamsInboxEntries verifies that a peer with
// inbox entries (entries received from third parties) can serve them
// over /agentfm/inbox-fetch/1.0.0 to a client that walks the local
// inbox with rowid pagination.
func TestInboxFetch_ServerStreamsInboxEntries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 2)
	server, client := hosts[0], hosts[1]

	keyServer, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	srvDB := filepath.Join(t.TempDir(), "server.db")
	srvLedger, err := ledger.NewWithOptions(srvDB, keyServer, nil, ledger.Options{Host: server})
	if err != nil {
		t.Fatalf("server ledger: %v", err)
	}
	t.Cleanup(func() { _ = srvLedger.Close() })

	keyRater, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen rater: %v", err)
	}
	raterID, err := peer.IDFromPrivateKey(keyRater)
	if err != nil {
		t.Fatalf("derive rater id: %v", err)
	}
	var prev [32]byte
	for i := byte(1); i <= 3; i++ {
		entry := &pb.SignedEntry{
			Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
				RaterPeerId:     []byte(raterID),
				SubjectPeerId:   []byte("subject"),
				Dimension:       "honesty",
				Score:           float64(i) / 10,
				TimestampUnixNs: time.Now().UnixNano(),
				PrevHash:        prev[:],
			}},
		}
		payload, err := proto.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal %d: %v", i, err)
		}
		var hash [32]byte
		hash[0] = i
		if err := srvLedger.Store().InsertInboxEntry(ctx, []byte(raterID), hash, prev, payload); err != nil {
			t.Fatalf("seed inbox %d: %v", i, err)
		}
		prev = hash
	}

	entries, err := ledger.FetchInboxFrom(ctx, client, server.ID(), 0, 10)
	if err != nil {
		t.Fatalf("fetch inbox: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Rowid >= entries[1].Rowid || entries[1].Rowid >= entries[2].Rowid {
		t.Fatalf("rowids not strictly increasing: %v", entries)
	}
	for i, e := range entries {
		if len(e.Payload) == 0 {
			t.Fatalf("entry %d has empty payload", i)
		}
	}

	cursor := entries[1].Rowid
	page2, err := ledger.FetchInboxFrom(ctx, client, server.ID(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch page 2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page 2 expected 1 entry, got %d", len(page2))
	}
	if page2[0].Rowid != entries[2].Rowid {
		t.Fatalf("page 2 rowid mismatch: got %d, want %d", page2[0].Rowid, entries[2].Rowid)
	}

	_ = network.InboxFetchProtocol
}

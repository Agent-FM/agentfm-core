package testutil

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// OpenTestStore opens a fresh SQLite-backed store in a per-test temp dir.
// The store is closed automatically via t.Cleanup.
func OpenTestStore(t testing.TB) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("testutil.OpenTestStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// AppendOwnRating inserts a Rating entry into the store's OWN log
// (the `entries` table, not inbox). This simulates ratings issued by the
// local peer — the same path the Boss uses for machine-issued attestation
// ratings. Uses the simplest possible non-zero hash derivation that keeps
// the store's uniqueness constraints happy.
func AppendOwnRating(t testing.TB, s *store.Store, rater host.Host, subject peer.ID, score float64, context string) {
	t.Helper()
	now := time.Now().UnixNano()
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.ID()),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		Context:         context,
		TimestampUnixNs: now,
		PrevHash:        make([]byte, 32),
	}}}
	payload, err := proto.Marshal(entry)
	if err != nil {
		t.Fatalf("testutil.AppendOwnRating marshal: %v", err)
	}
	// Hash = first 32 bytes of payload (or zero-padded). Unique enough for tests.
	var hash, prev [32]byte
	copy(hash[:], payload)
	_, err = s.AppendEntry(ctx2(), hash, prev, store.KindRating, payload, []byte{})
	if err != nil {
		t.Fatalf("testutil.AppendOwnRating AppendEntry: %v", err)
	}
}

// AppendInboxRating inserts a Rating entry into the store's INBOX log
// (the `inbox_entries` table). This simulates ratings received from another
// peer over gossip — the same path the boss uses after catching up with a
// remote rater's chain.
func AppendInboxRating(t testing.TB, s *store.Store, rater host.Host, subject peer.ID, score float64, context string) {
	t.Helper()
	now := time.Now().UnixNano()
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.ID()),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		Context:         context,
		TimestampUnixNs: now,
		PrevHash:        make([]byte, 32),
	}}}
	payload, err := proto.Marshal(entry)
	if err != nil {
		t.Fatalf("testutil.AppendInboxRating marshal: %v", err)
	}
	var hash [32]byte
	copy(hash[:], payload)
	if err := s.InsertInboxEntry(ctx2(), []byte(rater.ID()), hash, [32]byte{}, payload); err != nil {
		t.Fatalf("testutil.AppendInboxRating InsertInboxEntry: %v", err)
	}
}

func ctx2() context.Context { return context.Background() }

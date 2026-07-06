package boss

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/test/testutil"

	"google.golang.org/protobuf/proto"
)

func TestGatherPeerEntries_PopulatesEntryHash(t *testing.T) {
	ctx := context.Background()

	s := testutil.OpenTestStore(t)
	rater := testutil.NewHost(t)
	subject := testutil.NewHost(t)

	appendedEntry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.ID()),
		SubjectPeerId:   []byte(subject.ID()),
		Dimension:       "honesty",
		Score:           0.42,
		Context:         "entryhash-test",
		TimestampUnixNs: time.Now().UnixNano(),
		PrevHash:        make([]byte, 32),
	}}}
	payload, err := proto.Marshal(appendedEntry)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	var hash, prev [32]byte
	copy(hash[:], payload)
	if _, err := s.AppendEntry(ctx, hash, prev, store.KindRating, payload, []byte{}); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := GatherPeerEntries(ctx, s, subject.ID(), 10)
	if err != nil {
		t.Fatalf("GatherPeerEntries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected >=1 entry, got 0")
	}
	got := entries[0].EntryHash
	if len(got) != 64 {
		t.Fatalf("EntryHash hex length = %d (%q), want 64", len(got), got)
	}
	h := ledger.EntryHash(appendedEntry)
	if want := hex.EncodeToString(h[:]); got != want {
		t.Fatalf("EntryHash = %q, want %q (must match ledger.EntryHash of seeded entry)", got, want)
	}
}

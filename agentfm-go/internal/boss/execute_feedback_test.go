package boss

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"google.golang.org/protobuf/proto"
)

// newTestBossWithLedger creates a Boss wired with a real ledger and
// comments store in a temp dir. It also returns a second store handle
// on the same DB file so callers can iterate entries via
// IterateAllOwnEntries (WAL mode supports concurrent readers).
func newTestBossWithLedger(t *testing.T) (*Boss, *store.Store) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ledger.db")
	commentsDir := filepath.Join(dir, "comments")

	// Generate a fresh Ed25519 key for this Boss.
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}

	// Create the ledger (local-only: nil pubsub is fine for unit tests).
	l, err := ledger.New(dbPath, priv, nil)
	if err != nil {
		t.Fatalf("ledger.New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// Open a secondary read handle on the same file so tests can iterate.
	readStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open (read handle): %v", err)
	}
	t.Cleanup(func() { _ = readStore.Close() })

	// Wire the comments store.
	cs, err := comments.Open(commentsDir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	// Build the Boss. We need a real libp2p host so b.node.Host.ID() works.
	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		ledger:        l,
		commentsStore: cs,
	}

	return b, readStore
}

// TestAppendFeedbackComment_WritesCommentAndRating verifies that passing a
// non-nil rating score results in exactly one Comment entry and one Rating
// entry in the own ledger log.
func TestAppendFeedbackComment_WritesCommentAndRating(t *testing.T) {
	b, readStore := newTestBossWithLedger(t)
	subj := testutil.NewHost(t).ID()
	score := 0.5
	if err := b.appendFeedbackComment(context.Background(), subj, "task_abc", "great", &score); err != nil {
		t.Fatal(err)
	}

	var commentCount, ratingCount int
	_ = readStore.IterateAllOwnEntries(context.Background(), func(e *store.Entry) error {
		var s pb.SignedEntry
		_ = proto.Unmarshal(e.Payload, &s)
		if s.GetComment() != nil {
			commentCount++
		}
		if s.GetRating() != nil {
			ratingCount++
		}
		return nil
	})
	if commentCount != 1 || ratingCount != 1 {
		t.Fatalf("want 1 comment + 1 rating; got %d + %d", commentCount, ratingCount)
	}
}

// TestAppendFeedbackComment_NilRatingWritesOnlyComment verifies that passing
// a nil rating score writes only a Comment — no Rating entry.
func TestAppendFeedbackComment_NilRatingWritesOnlyComment(t *testing.T) {
	b, readStore := newTestBossWithLedger(t)
	subj := testutil.NewHost(t).ID()
	if err := b.appendFeedbackComment(context.Background(), subj, "task_a", "ok", nil); err != nil {
		t.Fatal(err)
	}

	var ratingCount int
	_ = readStore.IterateAllOwnEntries(context.Background(), func(e *store.Entry) error {
		var s pb.SignedEntry
		_ = proto.Unmarshal(e.Payload, &s)
		if s.GetRating() != nil {
			ratingCount++
		}
		return nil
	})
	if ratingCount != 0 {
		t.Fatalf("expected no rating entry when score=nil; got %d", ratingCount)
	}
}

// TestAppendFeedbackComment_NoLedger verifies the nil-ledger guard.
func TestAppendFeedbackComment_NoLedger(t *testing.T) {
	b := newTestBoss(t)
	// b.ledger is nil by default from newTestBoss.
	err := b.appendFeedbackComment(context.Background(), testutil.NewHost(t).ID(), "task_x", "hi", nil)
	if err == nil {
		t.Fatal("expected error when ledger is nil")
	}
}

// TestAppendFeedbackComment_NoCommentsStore verifies the nil-commentsStore guard.
func TestAppendFeedbackComment_NoCommentsStore(t *testing.T) {
	b := newTestBoss(t)
	b.ledger = &stubLedger{}
	// b.commentsStore is nil.
	err := b.appendFeedbackComment(context.Background(), testutil.NewHost(t).ID(), "task_x", "hi", nil)
	if err == nil {
		t.Fatal("expected error when commentsStore is nil")
	}
}

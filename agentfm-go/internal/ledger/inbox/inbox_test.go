package inbox_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/inbox"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func ctxBg() context.Context { return context.Background() }

type identity struct {
	priv crypto.PrivKey
	id   peer.ID
}

func newIdentity(t *testing.T) identity {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("derive peer id: %v", err)
	}
	return identity{priv: priv, id: pid}
}

// openInboxFor sets up an inbox owned by `own.id` backed by a fresh
// SQLite store. The store is closed via t.Cleanup.
func openInboxFor(t *testing.T, own identity) (*inbox.Inbox, *store.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inbox.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return inbox.New(s, own.id, 0, ledger.VerifyEntry, ledger.EntryHash), s
}

// signedRatingFromIdentity returns a SignedEntry whose Rating is authored
// by `rater` with the given prev_hash already on the chain.
func signedRatingFromIdentity(t *testing.T, rater identity, prev [32]byte, dim string) *pb.SignedEntry {
	t.Helper()
	e := &pb.SignedEntry{
		Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     []byte(rater.id),
			SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
			Dimension:       dim,
			Score:           0.1,
			Context:         "test",
			TimestampUnixNs: time.Now().UnixNano(),
		}},
	}
	if err := ledger.SignEntry(rater.priv, e, prev); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return e
}

// -----------------------------------------------------------------------------
// happy path
// -----------------------------------------------------------------------------

func TestAcceptOrQueue_ValidFirstEntry_Accepted(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	entry := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	if err := in.AcceptOrQueue(ctxBg(), entry); err != nil {
		t.Fatalf("AcceptOrQueue: %v", err)
	}

	h := ledger.EntryHash(entry)
	got, err := in.HasEntry(ctxBg(), []byte(rater.id), h)
	if err != nil {
		t.Fatalf("HasEntry: %v", err)
	}
	if !got {
		t.Fatal("entry not in inbox after Accept")
	}
}

func TestAcceptOrQueue_ChainExtension_Accepted(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e1 := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	if err := in.AcceptOrQueue(ctxBg(), e1); err != nil {
		t.Fatalf("Accept #1: %v", err)
	}
	h1 := ledger.EntryHash(e1)

	// Second entry: prev = hash of first
	e2 := signedRatingFromIdentity(t, rater, h1, "latency")
	if err := in.AcceptOrQueue(ctxBg(), e2); err != nil {
		t.Fatalf("Accept #2: %v", err)
	}
	h2 := ledger.EntryHash(e2)
	ok, err := in.HasEntry(ctxBg(), []byte(rater.id), h2)
	if err != nil {
		t.Fatalf("HasEntry: %v", err)
	}
	if !ok {
		t.Fatal("second entry not in inbox")
	}
}

// -----------------------------------------------------------------------------
// adversarial cases — every one MUST be silent (no panic) and result in
// the expected rejection / orphaning outcome.
// -----------------------------------------------------------------------------

func TestAcceptOrQueue_BadSignature_Rejected(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	// Flip a bit in the signature AFTER signing.
	e.GetRating().Signature[0] ^= 0x01

	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrSignatureInvalid) {
		t.Fatalf("want ErrSignatureInvalid, got %v", err)
	}

	// Inbox MUST NOT have stored the bad entry.
	h := ledger.EntryHash(e)
	if got, _ := in.HasEntry(ctxBg(), []byte(rater.id), h); got {
		t.Fatal("bad-signature entry was stored")
	}
}

func TestAcceptOrQueue_SelfMessage_Rejected(t *testing.T) {
	owner := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	// Author an entry as the OWNER identity — the inbox owner should
	// refuse to ingest its own outgoing entries.
	e := signedRatingFromIdentity(t, owner, [32]byte{}, "honesty")
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrSelfMessage) {
		t.Fatalf("want ErrSelfMessage, got %v", err)
	}
}

func TestAcceptOrQueue_ReplayDeduped(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	if err := in.AcceptOrQueue(ctxBg(), e); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	// Replay — same entry, second time. Must return nil (idempotent).
	if err := in.AcceptOrQueue(ctxBg(), e); err != nil {
		t.Fatalf("replay accept: %v", err)
	}
}

// -----------------------------------------------------------------------------
// orphan path: out-of-order entries are queued, then promoted when the
// parent arrives. Cap: an excess of out-of-order entries is rejected.
// -----------------------------------------------------------------------------

func TestAcceptOrQueue_OutOfOrder_Orphaned(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	// Build TWO entries chained 1 → 2 from rater. Deliver #2 first.
	e1 := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	h1 := ledger.EntryHash(e1)
	e2 := signedRatingFromIdentity(t, rater, h1, "latency")
	h2 := ledger.EntryHash(e2)

	// Out-of-order: deliver e2 first. e2's prev_hash references h1
	// which we haven't seen — must be orphaned.
	if err := in.AcceptOrQueue(ctxBg(), e2); err != nil {
		t.Fatalf("AcceptOrQueue e2: %v", err)
	}

	wasAccepted, _ := in.HasEntry(ctxBg(), []byte(rater.id), h2)
	if wasAccepted {
		t.Fatal("orphan was incorrectly accepted as entry")
	}
	isOrphan, _ := in.IsOrphan(ctxBg(), []byte(rater.id), h2)
	if !isOrphan {
		t.Fatal("expected entry to be queued as orphan")
	}
}

func TestAcceptOrQueue_OrphanPromotedWhenParentArrives(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e1 := signedRatingFromIdentity(t, rater, [32]byte{}, "honesty")
	h1 := ledger.EntryHash(e1)
	e2 := signedRatingFromIdentity(t, rater, h1, "latency")
	h2 := ledger.EntryHash(e2)
	e3 := signedRatingFromIdentity(t, rater, h2, "quality")
	h3 := ledger.EntryHash(e3)

	// Deliver out of order: 3, 2, then 1. After 1 arrives, both 2 and
	// 3 must be promoted out of the orphan queue.
	for _, e := range []*pb.SignedEntry{e3, e2} {
		if err := in.AcceptOrQueue(ctxBg(), e); err != nil {
			t.Fatalf("Accept out-of-order: %v", err)
		}
	}

	// Sanity: both orphaned before e1 arrives.
	if ok, _ := in.IsOrphan(ctxBg(), []byte(rater.id), h2); !ok {
		t.Fatal("e2 should be orphan before e1 arrives")
	}
	if ok, _ := in.IsOrphan(ctxBg(), []byte(rater.id), h3); !ok {
		t.Fatal("e3 should be orphan before e1 arrives")
	}

	// Deliver the parent. Should accept e1 and promote e2 → e3 in BFS.
	if err := in.AcceptOrQueue(ctxBg(), e1); err != nil {
		t.Fatalf("Accept e1: %v", err)
	}

	for _, hh := range [][32]byte{h1, h2, h3} {
		if ok, _ := in.HasEntry(ctxBg(), []byte(rater.id), hh); !ok {
			t.Errorf("expected hash %x in inbox after parent arrival", hh)
		}
		if ok, _ := in.IsOrphan(ctxBg(), []byte(rater.id), hh); ok {
			t.Errorf("hash %x still in orphan queue after promotion", hh)
		}
	}
}

func TestAcceptOrQueue_OrphanCapExceeded(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)

	// Small cap to keep the test fast.
	path := filepath.Join(t.TempDir(), "cap.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	in := inbox.New(s, owner.id, 3, ledger.VerifyEntry, ledger.EntryHash)

	// Deliver 3 unrelated orphans (each prev_hash points to a different
	// random hash that the rater never produces, so all stay orphaned).
	for i := 0; i < 3; i++ {
		prev := [32]byte{byte(i + 1), 0xff}
		e := signedRatingFromIdentity(t, rater, prev, "honesty")
		if err := in.AcceptOrQueue(ctxBg(), e); err != nil {
			t.Fatalf("orphan %d: %v", i, err)
		}
	}

	// 4th orphan must be rejected.
	prev := [32]byte{0x04, 0xff}
	e4 := signedRatingFromIdentity(t, rater, prev, "honesty")
	err = in.AcceptOrQueue(ctxBg(), e4)
	if !errors.Is(err, inbox.ErrOrphanCapExceeded) {
		t.Fatalf("want ErrOrphanCapExceeded, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// invariants
// -----------------------------------------------------------------------------

func TestAcceptOrQueue_NilEntry_ReturnsErrInvalidEntry(t *testing.T) {
	in, _ := openInboxFor(t, newIdentity(t))
	err := in.AcceptOrQueue(ctxBg(), nil)
	if !errors.Is(err, inbox.ErrInvalidEntry) {
		t.Fatalf("want ErrInvalidEntry, got %v", err)
	}
}

func TestAcceptOrQueue_EmptyBody_ReturnsErrInvalidEntry(t *testing.T) {
	in, _ := openInboxFor(t, newIdentity(t))
	err := in.AcceptOrQueue(ctxBg(), &pb.SignedEntry{})
	if !errors.Is(err, inbox.ErrInvalidEntry) {
		t.Fatalf("want ErrInvalidEntry, got %v", err)
	}
}

// Fix-3 acceptance: payload-level invariants are enforced AFTER
// signature verification so authenticated-but-out-of-range entries
// are still rejected.

func TestAcceptOrQueue_ScoreOutOfRange_ReturnsErrInvalidPayload(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.id),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       "honesty",
		Score:           42.0, // out of [-1, +1]
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if err := ledger.SignEntry(rater.priv, e, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrInvalidPayload) {
		t.Fatalf("want ErrInvalidPayload, got %v", err)
	}
}

func TestAcceptOrQueue_NaNScore_ReturnsErrInvalidPayload(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.id),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       "honesty",
		Score:           math.NaN(),
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if err := ledger.SignEntry(rater.priv, e, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrInvalidPayload) {
		t.Fatalf("want ErrInvalidPayload for NaN, got %v", err)
	}
}

func TestAcceptOrQueue_EmptyDimension_ReturnsErrInvalidPayload(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.id),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       "", // empty
		Score:           0.5,
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if err := ledger.SignEntry(rater.priv, e, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrInvalidPayload) {
		t.Fatalf("want ErrInvalidPayload for empty dimension, got %v", err)
	}
}

func TestAcceptOrQueue_BadTimestamp_ReturnsErrInvalidPayload(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater.id),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       "honesty",
		Score:           0,
		TimestampUnixNs: 0, // not positive
	}}}
	if err := ledger.SignEntry(rater.priv, e, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrInvalidPayload) {
		t.Fatalf("want ErrInvalidPayload for zero timestamp, got %v", err)
	}
}

func TestAcceptOrQueue_CommentMissingTextCID_ReturnsErrInvalidPayload(t *testing.T) {
	owner := newIdentity(t)
	rater := newIdentity(t)
	in, _ := openInboxFor(t, owner)

	e := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: &pb.Comment{
		RaterPeerId:     []byte(rater.id),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		TextCid:         nil, // missing
		Language:        "en",
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if err := ledger.SignEntry(rater.priv, e, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	err := in.AcceptOrQueue(ctxBg(), e)
	if !errors.Is(err, inbox.ErrInvalidPayload) {
		t.Fatalf("want ErrInvalidPayload for empty text_cid, got %v", err)
	}
}

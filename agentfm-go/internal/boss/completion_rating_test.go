package boss

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// completionRatingLedger is a recordingLedger that captures Appended entries
// so tests can assert what was written.
type completionRatingLedger struct {
	stubLedger
	appended []*pb.SignedEntry
}

func (l *completionRatingLedger) Append(_ context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	l.appended = append(l.appended, payload)
	return [32]byte{}, nil
}

// newFakePeerID generates a fresh libp2p peer ID for use in tests.
func newFakePeerID(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("IDFromPublicKey: %v", err)
	}
	return id
}

// newRatingWriter builds a CompletionRatingWriter backed by the given ledger
// and a fresh libp2p host (used only for its ID).
func newRatingWriter(t *testing.T, l *completionRatingLedger) *CompletionRatingWriter {
	t.Helper()
	h := testutil.NewHost(t)
	w := NewCompletionRatingWriter(l, h)
	return w
}

// TestCompletionRating_AggregatesAcrossWindow records 12 successes, advances
// the fake clock past the window, ticks, and asserts exactly one ledger entry
// with score +0.5 (capped) and context containing "successes=12,failures=0".
func TestCompletionRating_AggregatesAcrossWindow(t *testing.T) {
	l := &completionRatingLedger{}
	w := newRatingWriter(t, l)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.SetClockForTest(func() time.Time { return base })
	w.SetWindowForTest(time.Hour)

	subject := newFakePeerID(t)

	for i := 0; i < 12; i++ {
		w.RecordOutcome(subject, OutcomeSuccess)
	}

	// Advance clock past the 1h window.
	past := base.Add(time.Hour + time.Second)
	w.SetClockForTest(func() time.Time { return past })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(l.appended) != 1 {
		t.Fatalf("expected 1 appended entry; got %d", len(l.appended))
	}
	rating := l.appended[0].GetRating()
	if rating == nil {
		t.Fatal("appended entry has no Rating body")
	}
	if rating.Score != HourlyCap {
		t.Errorf("score = %v; want %v", rating.Score, HourlyCap)
	}
	if !strings.Contains(rating.Context, "successes=12") {
		t.Errorf("context %q does not contain 'successes=12'", rating.Context)
	}
	if !strings.Contains(rating.Context, "failures=0") {
		t.Errorf("context %q does not contain 'failures=0'", rating.Context)
	}
}

// TestCompletionRating_CapsAtHalfPerHour records 10,000 successes and asserts
// the rating score is capped at exactly +0.5.
func TestCompletionRating_CapsAtHalfPerHour(t *testing.T) {
	l := &completionRatingLedger{}
	w := newRatingWriter(t, l)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.SetClockForTest(func() time.Time { return base })
	w.SetWindowForTest(time.Hour)

	subject := newFakePeerID(t)

	for i := 0; i < 10_000; i++ {
		w.RecordOutcome(subject, OutcomeSuccess)
	}

	past := base.Add(time.Hour + time.Second)
	w.SetClockForTest(func() time.Time { return past })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(l.appended) != 1 {
		t.Fatalf("expected 1 appended entry; got %d", len(l.appended))
	}
	if got := l.appended[0].GetRating().Score; got != HourlyCap {
		t.Errorf("score = %v; want exactly %v (cap)", got, HourlyCap)
	}
}

// TestCompletionRating_NegativeOnFailures records 5 failures and asserts
// one entry with score -0.5 and context "failures=5,successes=0".
func TestCompletionRating_NegativeOnFailures(t *testing.T) {
	l := &completionRatingLedger{}
	w := newRatingWriter(t, l)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.SetClockForTest(func() time.Time { return base })
	w.SetWindowForTest(time.Hour)

	subject := newFakePeerID(t)

	for i := 0; i < 5; i++ {
		w.RecordOutcome(subject, OutcomeFailure)
	}

	past := base.Add(time.Hour + time.Second)
	w.SetClockForTest(func() time.Time { return past })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(l.appended) != 1 {
		t.Fatalf("expected 1 appended entry; got %d", len(l.appended))
	}
	rating := l.appended[0].GetRating()
	if rating.Score != -HourlyCap {
		t.Errorf("score = %v; want %v", rating.Score, -HourlyCap)
	}
	if !strings.Contains(rating.Context, "failures=5") {
		t.Errorf("context %q does not contain 'failures=5'", rating.Context)
	}
	if !strings.Contains(rating.Context, "successes=0") {
		t.Errorf("context %q does not contain 'successes=0'", rating.Context)
	}
}

// TestCompletionRating_NetZeroSkipsWrite records 3 successes + 3 failures
// (net 0.0) and asserts NO entry is appended.
func TestCompletionRating_NetZeroSkipsWrite(t *testing.T) {
	l := &completionRatingLedger{}
	w := newRatingWriter(t, l)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.SetClockForTest(func() time.Time { return base })
	w.SetWindowForTest(time.Hour)

	subject := newFakePeerID(t)

	for i := 0; i < 3; i++ {
		w.RecordOutcome(subject, OutcomeSuccess)
		w.RecordOutcome(subject, OutcomeFailure)
	}

	past := base.Add(time.Hour + time.Second)
	w.SetClockForTest(func() time.Time { return past })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(l.appended) != 0 {
		t.Errorf("expected 0 appended entries on net-zero; got %d", len(l.appended))
	}
	// Verify bucket was cleared.
	s, f := w.PendingForTest(subject)
	if s != 0 || f != 0 {
		t.Errorf("bucket not cleared after net-zero: successes=%d failures=%d", s, f)
	}
}

// TestCompletionRating_WindowGate records 12 successes, advances the clock
// only 30 minutes (within the window), asserts NO entry, then advances
// past the window and asserts one entry appears.
func TestCompletionRating_WindowGate(t *testing.T) {
	l := &completionRatingLedger{}
	w := newRatingWriter(t, l)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.SetClockForTest(func() time.Time { return base })
	w.SetWindowForTest(time.Hour)

	subject := newFakePeerID(t)

	for i := 0; i < 12; i++ {
		w.RecordOutcome(subject, OutcomeSuccess)
	}

	// Advance only 30 minutes — within the 1h window.
	half := base.Add(30 * time.Minute)
	w.SetClockForTest(func() time.Time { return half })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick (before window): %v", err)
	}
	if len(l.appended) != 0 {
		t.Errorf("expected 0 entries before window elapsed; got %d", len(l.appended))
	}

	// Advance past the window.
	past := base.Add(time.Hour + time.Minute)
	w.SetClockForTest(func() time.Time { return past })

	if err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick (after window): %v", err)
	}
	if len(l.appended) != 1 {
		t.Errorf("expected 1 entry after window elapsed; got %d", len(l.appended))
	}
}

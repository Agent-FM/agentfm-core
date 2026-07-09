package boss

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Outcome represents whether a dispatched task succeeded or failed.
type Outcome int

const (
	// OutcomeSuccess indicates the worker completed the task stream cleanly.
	OutcomeSuccess Outcome = iota
	// OutcomeFailure indicates the dispatch failed (dial fail, deadline, ghosted).
	OutcomeFailure
)

const (
	// DefaultRatingWindow is the interval between aggregate rating emissions.
	DefaultRatingWindow = time.Hour
	// PositiveIncrement is the per-success score contribution.
	PositiveIncrement = 0.1
	// NegativeIncrement is the per-failure score contribution (negative).
	NegativeIncrement = -0.1
	// HourlyCap is the maximum absolute score change per window.
	HourlyCap = 0.5
)

// peerBuckets accumulates success/failure counts for one peer within the
// current window.
type peerBuckets struct {
	successes, failures int
	lastEmit            time.Time
}

// CompletionRatingWriter accumulates dispatch outcomes in memory and, once per
// window (default: 1 hour), writes ONE signed Rating entry per peer into the
// ledger. The score is capped at ±HourlyCap to prevent retry-loop trust
// manufacturing.
//
// All methods are goroutine-safe.
type CompletionRatingWriter struct {
	mu     sync.Mutex
	ledger ledger.Ledger
	host   host.Host
	now    func() time.Time
	window time.Duration
	state  map[peer.ID]*peerBuckets
}

// NewCompletionRatingWriter constructs a writer that persists aggregate ratings
// into l and stamps them with h.ID() as the rater peer ID.
func NewCompletionRatingWriter(l ledger.Ledger, h host.Host) *CompletionRatingWriter {
	return &CompletionRatingWriter{
		ledger: l,
		host:   h,
		now:    time.Now,
		window: DefaultRatingWindow,
		state:  make(map[peer.ID]*peerBuckets),
	}
}

// RecordOutcome records one dispatch result for subject. Safe to call
// concurrently from multiple HTTP handler goroutines.
func (w *CompletionRatingWriter) RecordOutcome(subject peer.ID, o Outcome) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b := w.state[subject]
	if b == nil {
		// Initialise lastEmit to "now" so the first window starts from the
		// time the peer first appears, not from the zero epoch.
		b = &peerBuckets{lastEmit: w.now()}
		w.state[subject] = b
	}
	if o == OutcomeSuccess {
		b.successes++
	} else {
		b.failures++
	}
}

// Tick scans all peer buckets. For each peer whose last emission is older
// than the window, it computes the capped aggregate score and—if non-zero—
// appends a signed Rating to the ledger. Net-zero buckets are cleared without
// writing.
func (w *CompletionRatingWriter) Tick(ctx context.Context) error {
	type pending struct {
		entry         *pb.SignedEntry
		bucket        *peerBuckets
		emitSuccesses int
		emitFailures  int
	}

	w.mu.Lock()
	now := w.now()
	var toWrite []pending
	for pid, b := range w.state {
		if now.Sub(b.lastEmit) < w.window {
			continue
		}
		score := computeAggregateScore(b)
		if score == 0 {
			b.successes, b.failures = 0, 0
			continue
		}
		rating := &pb.Rating{
			RaterPeerId:   []byte(w.host.ID()),
			SubjectPeerId: []byte(pid),
			Dimension:     "honesty",
			Score:         score,
			Context: fmt.Sprintf(
				"task:successes=%d,failures=%d,window=%ds",
				b.successes, b.failures, int(w.window.Seconds()),
			),
			TimestampUnixNs: now.UnixNano(),
		}
		toWrite = append(toWrite, pending{
			entry:         &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}},
			bucket:        b,
			emitSuccesses: b.successes,
			emitFailures:  b.failures,
		})
	}
	w.mu.Unlock()

	for _, p := range toWrite {
		if _, err := w.ledger.Append(ctx, p.entry); err != nil {
			return err
		}
		w.mu.Lock()
		p.bucket.successes -= p.emitSuccesses
		p.bucket.failures -= p.emitFailures
		p.bucket.lastEmit = now
		w.mu.Unlock()
	}
	return nil
}

// netZeroEpsilon is the tolerance below which a raw score is treated as zero
// and the bucket is cleared without writing. Using integer arithmetic avoids
// floating-point drift: net = successes - failures, each weighted equally.
const netZeroEpsilon = 1e-9

// computeAggregateScore converts raw success/failure counts to a score clamped
// to [-HourlyCap, +HourlyCap]. Returns 0 when the net contribution rounds to
// zero (i.e. equal successes and failures with symmetric increments).
func computeAggregateScore(b *peerBuckets) float64 {
	// Use integer net to avoid floating-point drift (PositiveIncrement ==
	// -NegativeIncrement, so net successes == net failures means exactly 0).
	net := b.successes - b.failures
	if net == 0 {
		return 0
	}
	raw := PositiveIncrement*float64(b.successes) + NegativeIncrement*float64(b.failures)
	if raw > -netZeroEpsilon && raw < netZeroEpsilon {
		return 0
	}
	if raw > HourlyCap {
		return HourlyCap
	}
	if raw < -HourlyCap {
		return -HourlyCap
	}
	return raw
}

// RunTicker calls Tick on a window-sized interval until ctx is canceled. It is
// intended to be launched as a background goroutine:
//
//	go w.RunTicker(ctx)
func (w *CompletionRatingWriter) RunTicker(ctx context.Context) {
	t := time.NewTicker(w.window)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = w.Tick(ctx)
		}
	}
}

// --- Test helpers -----------------------------------------------------------
// These are exported (capitalised receiver methods) so the in-package test
// file can drive them without build tags or separate _internal packages.

// SetClockForTest replaces the internal time source. Useful for deterministic
// window-gate tests.
func (w *CompletionRatingWriter) SetClockForTest(f func() time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.now = f
}

// SetWindowForTest overrides the emission window. Allows tests to use shorter
// windows without waiting real-time durations.
func (w *CompletionRatingWriter) SetWindowForTest(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.window = d
}

// PendingForTest returns the current pending success/failure counts for a peer.
// Returns (0, 0) if the peer has no bucket.
func (w *CompletionRatingWriter) PendingForTest(p peer.ID) (successes, failures int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if b, ok := w.state[p]; ok {
		return b.successes, b.failures
	}
	return 0, 0
}

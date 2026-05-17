package reputation_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/reputation"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// rig wraps a fresh store + helpers for rating insertion.
type rig struct {
	t     *testing.T
	store *store.Store
	now   time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "rep.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &rig{t: t, store: s, now: time.Now()}
}

func newPID(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, _ := peer.IDFromPublicKey(pub)
	return id
}

// insertRating writes a SignedEntry/Rating into the inbox. We bypass
// the full signing path because EigenTrust doesn't re-verify; it
// just reads scores and weights.
func (r *rig) insertRating(rater, subject peer.ID, score float64, ageDays float64) {
	r.t.Helper()
	ts := r.now.Add(-time.Duration(ageDays * 24 * float64(time.Hour))).UnixNano()
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		TimestampUnixNs: ts,
		PrevHash:        make([]byte, 32),
	}}}
	payload, _ := proto.Marshal(entry)
	var hash [32]byte
	copy(hash[:], payload[:32])
	if err := r.store.InsertInboxEntry(context.Background(), []byte(rater), hash, [32]byte{}, payload); err != nil {
		r.t.Fatalf("InsertInboxEntry: %v", err)
	}
}

func TestEigenTrust_SeedOnly_PreservesSeedScores(t *testing.T) {
	r := newRig(t)
	a := newPID(t)
	eng := reputation.New([]reputation.Seed{{PeerID: a.String(), Score: 0.9}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	got := eng.Score(a.String())
	if got != 0.9 {
		t.Errorf("seed score not preserved: %v", got)
	}
}

func TestEigenTrust_PositiveRatingsFromSeed_PropagatesTrust(t *testing.T) {
	r := newRig(t)
	seed := newPID(t)
	target := newPID(t)
	r.insertRating(seed, target, 1.0, 1.0) // fresh +1.0 from a seed peer
	eng := reputation.New([]reputation.Seed{{PeerID: seed.String(), Score: 1.0}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	got := eng.Score(target.String())
	if got <= 0 {
		t.Errorf("target should have positive score from a +1.0 rating by seed; got %v", got)
	}
}

func TestEigenTrust_SybilFlood_DoesNotMoveScore(t *testing.T) {
	// 50 sybil peers (no seed weight) all -1.0 an honest peer.
	// Without seed weight, their iteration weight is 0, so the
	// target's score should remain near 0 (no movement).
	r := newRig(t)
	honest := newPID(t)
	for i := 0; i < 50; i++ {
		sybil := newPID(t)
		r.insertRating(sybil, honest, -1.0, 1.0)
	}
	eng := reputation.New(nil, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	got := eng.Score(honest.String())
	if got < -0.5 {
		t.Errorf("sybil flood pushed honest peer below -0.5: got %v", got)
	}
}

func TestEigenTrust_AgeDecayDilutesOldRatings(t *testing.T) {
	r := newRig(t)
	seed := newPID(t)
	subject := newPID(t)
	// One ancient +1.0 from a seed (60 days old, two half-lives).
	r.insertRating(seed, subject, 1.0, 60.0)
	eng := reputation.New([]reputation.Seed{{PeerID: seed.String(), Score: 1.0}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	score := eng.Score(subject.String())
	// 60-day half-life means age weight ≈ 0.25; with mixing 0.15,
	// the seed-side contribution dominates the convergence. The
	// score should be POSITIVE but visibly attenuated vs a fresh
	// rating.
	if score <= 0 {
		t.Errorf("aged positive rating produced non-positive score: %v", score)
	}
	if score > 0.95 {
		t.Errorf("aged rating produced near-max score: %v (decay not applied?)", score)
	}
}

func TestShouldEject_Hysteresis(t *testing.T) {
	if !reputation.ShouldEject(-0.6) {
		t.Errorf("score -0.6 should be ejected")
	}
	if reputation.ShouldEject(-0.5) {
		t.Errorf("score -0.5 is at the threshold; ShouldEject must return false (strict <)")
	}
	if !reputation.CanReadmit(-0.3) {
		t.Errorf("score -0.3 should be re-admittable")
	}
	if reputation.CanReadmit(-0.4) {
		t.Errorf("score -0.4 should NOT be re-admittable (hysteresis)")
	}
}

func TestSnapshot_IsCopy(t *testing.T) {
	r := newRig(t)
	seed := newPID(t)
	eng := reputation.New([]reputation.Seed{{PeerID: seed.String(), Score: 0.5}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	snap := eng.Snapshot()
	snap[seed.String()] = -99 // mutate the snapshot
	if eng.Score(seed.String()) == -99 {
		t.Fatal("Snapshot returned a live reference; should be a copy")
	}
}

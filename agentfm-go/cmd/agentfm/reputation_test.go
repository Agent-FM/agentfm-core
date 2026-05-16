package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// renderer-only tests (no SQLite, no subprocess)
// -----------------------------------------------------------------------------

func TestRenderReputationView_EmptyView(t *testing.T) {
	subj, _ := mintPeerID(t)
	var buf bytes.Buffer
	renderReputationView(&buf, &reputationView{Subject: subj})
	out := buf.String()
	if !strings.Contains(out, "Entries:    0") {
		t.Errorf("empty view should announce zero entries; got:\n%s", out)
	}
	if !strings.Contains(out, "Honesty:    [no data]") {
		t.Errorf("empty view should mark Honesty as [no data]; got:\n%s", out)
	}
}

func TestRenderReputationView_PopulatedView(t *testing.T) {
	subj, _ := mintPeerID(t)
	rater, _ := mintPeerID(t)
	now := time.Now()
	view := &reputationView{
		Subject:    subj,
		EntryCount: 3,
		LastSeen:   now.Add(-2 * time.Minute),
		LatestEntries: []reputationRow{
			{ReceivedAt: now.Add(-2 * time.Minute), Rater: rater, Dimension: "honesty", Score: +0.10, Context: "probe_ok", Kind: "Rating"},
			{ReceivedAt: now.Add(-14 * time.Minute), Rater: rater, Dimension: "honesty", Score: -0.70, Context: "probe_fail", Kind: "Rating"},
			{ReceivedAt: now.Add(-3 * time.Hour), Rater: rater, Kind: "Comment", Context: "en"},
		},
	}
	var buf bytes.Buffer
	renderReputationView(&buf, view)
	out := buf.String()

	wants := []string{
		subj.String(),
		"Entries:    3 (last:",
		"Honesty:    [pending P3-7",
		"+0.10 honesty",
		"-0.70 honesty",
		"comment by",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("rendered output missing %q\nfull:\n%s", w, out)
		}
	}
}

func TestShortPeer_TruncatesLongIDs(t *testing.T) {
	id, _ := mintPeerID(t)
	short := shortPeer(id)
	if len(short) != 14 { // 6 + "..." + 5
		t.Errorf("shortPeer length = %d, want 14; got %q", len(short), short)
	}
	if !strings.Contains(short, "...") {
		t.Errorf("shortPeer should contain ellipsis; got %q", short)
	}
}

func TestCompactAge_FormatBoundaries(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{59 * time.Minute, "59m"},
		{60 * time.Minute, "1h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{-1 * time.Second, "soon"},
	}
	for _, tc := range cases {
		if got := compactAge(tc.d); got != tc.want {
			t.Errorf("compactAge(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// gather test: populate an inbox via a real Ledger, then assert the
// gathered view reflects only entries about the target subject.
// -----------------------------------------------------------------------------

func TestGatherReputationView_FiltersBySubject(t *testing.T) {
	owner, ownerPriv := mintPeerID(t)
	raterA, raterAPriv := mintPeerID(t)
	raterB, raterBPriv := mintPeerID(t)
	subject, _ := mintPeerID(t)
	otherSubject, _ := mintPeerID(t)
	_ = owner
	_ = ownerPriv

	// Open a fresh store; create a Ledger to drive Append on raterA's
	// behalf so the inbox accumulates entries we can query later. The
	// path here is the SAME store we'll read from in gather — the
	// Ledger writes via the store, the CLI reads via the store.
	path := filepath.Join(t.TempDir(), "ledger.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Bypass the gossip path: insert entries directly into the inbox
	// by serialising signed envelopes and calling InsertInboxEntry.
	insertEntry := func(t *testing.T, raterPriv crypto.PrivKey, rater peer.ID, subj peer.ID, prev [32]byte, dim string, score float64) [32]byte {
		t.Helper()
		entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     []byte(rater),
			SubjectPeerId:   []byte(subj),
			Dimension:       dim,
			Score:           score,
			TimestampUnixNs: time.Now().UnixNano(),
		}}}
		if err := ledger.SignEntry(raterPriv, entry, prev); err != nil {
			t.Fatalf("sign: %v", err)
		}
		hash := ledger.EntryHash(entry)
		payload, err := protoMarshalForTest(entry)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := s.InsertInboxEntry(context.Background(), []byte(rater), hash, prev, payload); err != nil {
			t.Fatalf("InsertInboxEntry: %v", err)
		}
		return hash
	}

	// Three entries about subject (across two raters), one about otherSubject.
	hA1 := insertEntry(t, raterAPriv, raterA, subject, [32]byte{}, "honesty", 0.5)
	insertEntry(t, raterAPriv, raterA, subject, hA1, "latency", 0.7)
	insertEntry(t, raterBPriv, raterB, subject, [32]byte{}, "honesty", -0.3)
	insertEntry(t, raterBPriv, raterB, otherSubject, [32]byte{}, "honesty", 0.1)

	view, err := gatherReputationView(context.Background(), s, []byte(subject), 10)
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if view.EntryCount != 3 {
		t.Errorf("EntryCount = %d, want 3", view.EntryCount)
	}
	if len(view.LatestEntries) != 3 {
		t.Errorf("LatestEntries len = %d, want 3", len(view.LatestEntries))
	}
	if view.Subject != subject {
		t.Errorf("Subject = %s, want %s", view.Subject, subject)
	}
}

func TestGatherReputationView_LimitTruncates(t *testing.T) {
	rater, raterPriv := mintPeerID(t)
	subject, _ := mintPeerID(t)
	path := filepath.Join(t.TempDir(), "ledger.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var prev [32]byte
	for i := 0; i < 5; i++ {
		entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     []byte(rater),
			SubjectPeerId:   []byte(subject),
			Dimension:       "honesty",
			Score:           float64(i) / 10,
			TimestampUnixNs: time.Now().UnixNano() + int64(i),
		}}}
		if err := ledger.SignEntry(raterPriv, entry, prev); err != nil {
			t.Fatalf("sign %d: %v", i, err)
		}
		h := ledger.EntryHash(entry)
		payload, _ := protoMarshalForTest(entry)
		if err := s.InsertInboxEntry(context.Background(), []byte(rater), h, prev, payload); err != nil {
			t.Fatalf("InsertInboxEntry %d: %v", i, err)
		}
		prev = h
	}

	view, err := gatherReputationView(context.Background(), s, []byte(subject), 2)
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if view.EntryCount != 5 {
		t.Errorf("EntryCount = %d, want 5 (full count regardless of limit)", view.EntryCount)
	}
	if len(view.LatestEntries) != 2 {
		t.Errorf("LatestEntries len = %d, want 2 (limited)", len(view.LatestEntries))
	}
}

// -----------------------------------------------------------------------------
// end-to-end smoke: build the binary, populate a DB, run the subcommand
// -----------------------------------------------------------------------------

func TestReputationShow_EndToEnd_PrintsExpectedSections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build under -short")
	}
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "agentfm")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build agentfm: %v\n%s", err, out)
	}

	// Populate a DB with one rating about the target subject.
	dbPath := filepath.Join(t.TempDir(), ".agentfm_ledger.db")
	rater, raterPriv := mintPeerID(t)
	subject, _ := mintPeerID(t)
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(rater),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           0.42,
		Context:         "smoke-test",
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if err := ledger.SignEntry(raterPriv, entry, [32]byte{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	hash := ledger.EntryHash(entry)
	payload, _ := protoMarshalForTest(entry)
	if err := s.InsertInboxEntry(context.Background(), []byte(rater), hash, [32]byte{}, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}
	_ = s.Close()

	cmd := exec.Command(binPath, "reputation", "show", "-db", dbPath, subject.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("agentfm reputation show: %v\n%s", err, out)
	}
	got := string(out)

	for _, w := range []string{
		"Peer:       " + subject.String(),
		"Entries:    1",
		"Honesty:    [pending P3-7",
		"+0.42 honesty",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("CLI output missing %q\nfull:\n%s", w, got)
		}
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func mintPeerID(t *testing.T) (peer.ID, crypto.PrivKey) {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("peer id: %v", err)
	}
	return id, priv
}

// protoMarshalForTest is a thin wrapper so the gather tests don't
// have to drag a third import into every fixture.
func protoMarshalForTest(e *pb.SignedEntry) ([]byte, error) {
	return proto.Marshal(e)
}

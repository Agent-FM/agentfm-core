package ledger

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// freshAlertImpl returns a *ledgerImpl backed by a fresh SQLite
// store. Returned directly (not via the public Ledger interface) so
// the test can call acceptLocalAlert without re-exposing it.
func freshAlertImpl(t *testing.T) *ledgerImpl {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatalf("peer id: %v", err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "alert.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &ledgerImpl{
		store:  s,
		key:    priv,
		peerID: pid,
	}
}

// witnessIdent + offenderIdent helpers — minimal Ed25519 identities
// for building alerts.
type alertIdent struct {
	priv crypto.PrivKey
	id   peer.ID
}

func newAlertIdent(t *testing.T) alertIdent {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("peer id: %v", err)
	}
	return alertIdent{priv: priv, id: id}
}

// signHeadFor signs a head at (treeSize, root) on behalf of signer.
func signHeadFor(t *testing.T, signer alertIdent, treeSize uint64, root []byte) *pb.LogHead {
	t.Helper()
	head := &pb.LogHead{
		PeerId:          []byte(signer.id),
		TreeSize:        treeSize,
		RootHash:        root,
		TimestampUnixNs: time.Now().UnixNano(),
	}
	canonical, err := pb.CanonicalLogHead(head)
	if err != nil {
		t.Fatalf("canonical head: %v", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := signer.priv.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign head: %v", err)
	}
	head.Signature = sig
	return head
}

// signAlert constructs an EquivocationAlert with the witness's own
// signature applied. headA and headB are passed in pre-signed by
// whoever they're attributed to (good or bad).
func signAlert(t *testing.T, witness, offender alertIdent, headA, headB *pb.LogHead) *pb.EquivocationAlert {
	t.Helper()
	alert := &pb.EquivocationAlert{
		PeerId:          []byte(offender.id),
		HeadA:           headA,
		HeadB:           headB,
		WitnessPeerId:   []byte(witness.id),
		TimestampUnixNs: time.Now().UnixNano(),
	}
	canonical, err := pb.CanonicalEquivocationAlert(alert)
	if err != nil {
		t.Fatalf("canonical alert: %v", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := witness.priv.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign alert: %v", err)
	}
	alert.WitnessSignature = sig
	return alert
}

// happy path: well-formed alert from a real witness against a real
// offender with two genuinely-conflicting heads gets accepted +
// marks the offender.
func TestAcceptLocalAlert_HappyPath_MarksOffender(t *testing.T) {
	l := freshAlertImpl(t)
	w := newAlertIdent(t)
	o := newAlertIdent(t)

	rootA := make([]byte, 32)
	rootA[0] = 0xaa
	rootB := make([]byte, 32)
	rootB[0] = 0xbb
	headA := signHeadFor(t, o, 5, rootA)
	headB := signHeadFor(t, o, 5, rootB)
	alert := signAlert(t, w, o, headA, headB)

	if err := l.acceptLocalAlert(context.Background(), alert); err != nil {
		t.Fatalf("acceptLocalAlert: %v", err)
	}
	marked, err := l.store.IsEquivocator(context.Background(), []byte(o.id))
	if err != nil {
		t.Fatalf("IsEquivocator: %v", err)
	}
	if !marked {
		t.Fatal("offender not marked after accepted alert")
	}
}

// Fix-2 acceptance: a rogue witness forges two unrelated heads it
// claims came from some other peer (offender O). The forged heads
// are NOT signed by O. acceptLocalAlert must refuse to mark O.
func TestAcceptLocalAlert_ForgedHeads_DoesNotMarkOffender(t *testing.T) {
	l := freshAlertImpl(t)
	rogueWitness := newAlertIdent(t)
	innocentOffender := newAlertIdent(t)
	imposter := newAlertIdent(t) // rogue witness signs heads pretending to be the offender

	rootA := make([]byte, 32)
	rootA[0] = 0xaa
	rootB := make([]byte, 32)
	rootB[0] = 0xbb
	// Heads "signed" by imposter, but the alert claims they're from innocentOffender.
	headA := signHeadFor(t, imposter, 5, rootA)
	headA.PeerId = []byte(innocentOffender.id) // post-sign tamper: claim innocent's identity
	headB := signHeadFor(t, imposter, 5, rootB)
	headB.PeerId = []byte(innocentOffender.id)
	alert := signAlert(t, rogueWitness, innocentOffender, headA, headB)

	err := l.acceptLocalAlert(context.Background(), alert)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	marked, _ := l.store.IsEquivocator(context.Background(), []byte(innocentOffender.id))
	if marked {
		t.Fatal("innocent offender wrongly marked by forged alert")
	}
}

// Fix-2 acceptance: HeadA == HeadB is not a real conflict; alert
// should NOT mark.
func TestAcceptLocalAlert_DuplicateHead_NoMark(t *testing.T) {
	l := freshAlertImpl(t)
	w := newAlertIdent(t)
	o := newAlertIdent(t)

	root := make([]byte, 32)
	root[0] = 0xab
	head := signHeadFor(t, o, 5, root)
	cloneA := proto.Clone(head).(*pb.LogHead)
	cloneB := proto.Clone(head).(*pb.LogHead)
	alert := signAlert(t, w, o, cloneA, cloneB)

	if err := l.acceptLocalAlert(context.Background(), alert); err != nil {
		t.Fatalf("acceptLocalAlert: %v", err)
	}
	marked, _ := l.store.IsEquivocator(context.Background(), []byte(o.id))
	if marked {
		t.Fatal("offender marked despite identical alert heads")
	}
}

// Fix-2 acceptance: alert with same (size, root) on both heads
// (semantically duplicate even if non-identical proto bytes) must
// not mark.
func TestAcceptLocalAlert_SameSizeSameRoot_NoMark(t *testing.T) {
	l := freshAlertImpl(t)
	w := newAlertIdent(t)
	o := newAlertIdent(t)

	root := make([]byte, 32)
	root[0] = 0xcd
	headA := signHeadFor(t, o, 5, root)
	headB := signHeadFor(t, o, 5, append([]byte(nil), root...)) // same root, different sig+timestamp
	// Ensure they're not byte-equal (different timestamps).
	if proto.Equal(headA, headB) {
		// If they happened to land in the same nanosecond, force a difference.
		headB.TimestampUnixNs++
		// Re-sign so the sig matches the tweaked timestamp.
		canonical, _ := pb.CanonicalLogHead(headB)
		digest := sha256.Sum256(canonical)
		sig, _ := o.priv.Sign(digest[:])
		headB.Signature = sig
	}
	alert := signAlert(t, w, o, headA, headB)

	err := l.acceptLocalAlert(context.Background(), alert)
	if err == nil {
		t.Fatal("expected error for same-size-same-root alert, got nil")
	}
	marked, _ := l.store.IsEquivocator(context.Background(), []byte(o.id))
	if marked {
		t.Fatal("offender marked despite no actual root conflict")
	}
}

// Fix-2 acceptance: a tampered witness signature on the alert
// itself must be rejected.
func TestAcceptLocalAlert_BadWitnessSig_DoesNotMark(t *testing.T) {
	l := freshAlertImpl(t)
	w := newAlertIdent(t)
	o := newAlertIdent(t)

	rootA := make([]byte, 32)
	rootA[0] = 0xaa
	rootB := make([]byte, 32)
	rootB[0] = 0xbb
	headA := signHeadFor(t, o, 5, rootA)
	headB := signHeadFor(t, o, 5, rootB)
	alert := signAlert(t, w, o, headA, headB)
	alert.WitnessSignature[0] ^= 0x01 // post-sign tamper

	err := l.acceptLocalAlert(context.Background(), alert)
	if err == nil {
		t.Fatal("expected error for tampered witness sig, got nil")
	}
	marked, _ := l.store.IsEquivocator(context.Background(), []byte(o.id))
	if marked {
		t.Fatal("offender marked despite tampered witness sig")
	}
}

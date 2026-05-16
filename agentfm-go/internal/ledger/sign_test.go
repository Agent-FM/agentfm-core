package ledger_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/merkle"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// freshIdentity returns a fresh Ed25519 keypair plus the peer ID derived
// from its public half. Mirrors the worker_identity.key bootstrap path:
// PeerID == PeerID.IDFromPublicKey(pubKey) and the bytes round-trip via
// peer.IDFromBytes(...).ExtractPublicKey().
func freshIdentity(t *testing.T) (crypto.PrivKey, crypto.PubKey, []byte) {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("derive peer id: %v", err)
	}
	return priv, pub, []byte(pid)
}

// newRatingFor returns an unsigned Rating addressed FROM raterID about
// some fabricated subject. Caller supplies prev_hash via SignEntry.
func newRatingFor(raterID []byte) *pb.Rating {
	return &pb.Rating{
		RaterPeerId:     raterID,
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       "honesty",
		Score:           0.5,
		Context:         "probe_round_42",
		TimestampUnixNs: 1_700_000_000_000_000_000,
	}
}

func newCommentFor(raterID []byte) *pb.Comment {
	return &pb.Comment{
		RaterPeerId:     raterID,
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		TextCid:         bytes.Repeat([]byte{0xab}, 34),
		Language:        "en",
		TimestampUnixNs: 1_700_000_000_000_000_000,
	}
}

func zeroPrev() [32]byte { return [32]byte{} }

// -----------------------------------------------------------------------------
// SignEntry sets PrevHash + Signature in place
// -----------------------------------------------------------------------------

func TestSignEntry_Rating_PopulatesPrevAndSig(t *testing.T) {
	priv, _, rid := freshIdentity(t)

	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}

	prev := [32]byte{0xaa, 0xbb, 0xcc}
	if err := ledger.SignEntry(priv, entry, prev); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	if !bytes.Equal(rating.PrevHash, prev[:]) {
		t.Errorf("PrevHash not set: got %x, want %x", rating.PrevHash, prev[:])
	}
	if len(rating.Signature) == 0 {
		t.Errorf("Signature not set")
	}
}

func TestSignEntry_Comment_PopulatesPrevAndSig(t *testing.T) {
	priv, _, rid := freshIdentity(t)

	comment := newCommentFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: comment}}

	prev := [32]byte{0x11, 0x22}
	if err := ledger.SignEntry(priv, entry, prev); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	if !bytes.Equal(comment.PrevHash, prev[:]) {
		t.Errorf("PrevHash not set")
	}
	if len(comment.Signature) == 0 {
		t.Errorf("Signature not set")
	}
}

// -----------------------------------------------------------------------------
// VerifyEntry round-trip
// -----------------------------------------------------------------------------

func TestVerifyEntry_RoundTrip_Rating(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRatingFor(rid)}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	ok, err := ledger.VerifyEntry(entry)
	if err != nil {
		t.Fatalf("VerifyEntry returned error: %v", err)
	}
	if !ok {
		t.Fatal("VerifyEntry returned false on freshly signed entry")
	}
}

func TestVerifyEntry_RoundTrip_Comment(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: newCommentFor(rid)}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	ok, err := ledger.VerifyEntry(entry)
	if err != nil {
		t.Fatalf("VerifyEntry returned error: %v", err)
	}
	if !ok {
		t.Fatal("VerifyEntry returned false on freshly signed Comment")
	}
}

// Sign with key A, but the entry claims rater_peer_id == PeerID(B). Verifier
// extracts B's public key and rejects A's signature.
func TestVerifyEntry_WrongIdentity_Rejected(t *testing.T) {
	privA, _, _ := freshIdentity(t)
	_, _, ridB := freshIdentity(t)

	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRatingFor(ridB)}}
	if err := ledger.SignEntry(privA, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	ok, err := ledger.VerifyEntry(entry)
	if err != nil {
		t.Fatalf("VerifyEntry returned operational error: %v", err)
	}
	if ok {
		t.Fatal("VerifyEntry accepted a signature from the wrong key")
	}
}

// -----------------------------------------------------------------------------
// tamper tests — flip a payload field after signing, verification fails
// -----------------------------------------------------------------------------

func TestVerifyEntry_FlipDimension_Fails(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	rating.Dimension = "latency" // post-sign mutation
	ok, _ := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry after Dimension was tampered")
	}
}

func TestVerifyEntry_FlipScore_Fails(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	rating.Score = -1.0
	ok, _ := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry after Score was tampered")
	}
}

func TestVerifyEntry_FlipPrevHash_Fails(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	rating.PrevHash[0] ^= 0x01
	ok, _ := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry after PrevHash was tampered")
	}
}

func TestVerifyEntry_FlipSubject_Fails(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	rating.SubjectPeerId[0] ^= 0xff
	ok, _ := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry after SubjectPeerId was tampered")
	}
}

func TestVerifyEntry_FlipSignature_Fails(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	rating := newRatingFor(rid)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	rating.Signature[0] ^= 0x01
	ok, _ := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry after Signature was tampered")
	}
}

// -----------------------------------------------------------------------------
// EntryHash semantics
// -----------------------------------------------------------------------------

func TestEntryHash_DeterministicAcrossCalls(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRatingFor(rid)}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	h1 := ledger.EntryHash(entry)
	for i := 0; i < 20; i++ {
		if h := ledger.EntryHash(entry); h != h1 {
			t.Fatalf("EntryHash non-deterministic at iter %d", i)
		}
	}
}

// EntryHash MUST equal merkle.HashLeaf(canonical_bytes). This is the
// contract that makes prev_hash chains line up with Merkle leaves so a
// verifier can use the same value to walk the chain or the tree.
func TestEntryHash_EqualsMerkleLeafOfCanonical(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRatingFor(rid)}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	canonical, err := pb.CanonicalSignedEntry(entry)
	if err != nil {
		t.Fatalf("CanonicalSignedEntry: %v", err)
	}
	want := merkle.HashLeaf(canonical)
	if got := ledger.EntryHash(entry); got != want {
		t.Fatalf("EntryHash != HashLeaf(canonical):\n got  %x\n want %x", got, want)
	}
}

// EntryHash and the signing digest are DIFFERENT values — the signing
// digest is SHA-256(canonical) (no leaf prefix), the entry hash uses
// HashLeaf which prepends 0x00. This is by design (domain separation
// between signed payloads and Merkle leaves). The test pins the
// distinction so a refactor that conflates them will fail loudly.
func TestEntryHash_DiffersFromBareSHA256(t *testing.T) {
	priv, _, rid := freshIdentity(t)
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRatingFor(rid)}}
	if err := ledger.SignEntry(priv, entry, zeroPrev()); err != nil {
		t.Fatalf("SignEntry: %v", err)
	}
	canonical, _ := pb.CanonicalSignedEntry(entry)
	bare := sha256.Sum256(canonical)
	entryHash := ledger.EntryHash(entry)
	if entryHash == bare {
		t.Fatal("EntryHash equals bare sha256(canonical) — domain separation broken")
	}
}

// -----------------------------------------------------------------------------
// error paths
// -----------------------------------------------------------------------------

func TestSignEntry_NilEntry_Errors(t *testing.T) {
	priv, _, _ := freshIdentity(t)
	if err := ledger.SignEntry(priv, nil, zeroPrev()); err == nil {
		t.Fatal("expected error for nil entry, got nil")
	}
}

func TestSignEntry_EmptyBody_Errors(t *testing.T) {
	priv, _, _ := freshIdentity(t)
	if err := ledger.SignEntry(priv, &pb.SignedEntry{}, zeroPrev()); err == nil {
		t.Fatal("expected error for SignedEntry with no body, got nil")
	}
}

func TestVerifyEntry_NilEntry_Errors(t *testing.T) {
	_, err := ledger.VerifyEntry(nil)
	if err == nil {
		t.Fatal("expected error for nil entry, got nil")
	}
}

func TestVerifyEntry_InvalidPeerID_Errors(t *testing.T) {
	rating := &pb.Rating{
		RaterPeerId:     []byte("not-a-valid-peer-id"),
		SubjectPeerId:   bytes.Repeat([]byte{0x01}, 32),
		Dimension:       "honesty",
		Score:           0,
		TimestampUnixNs: 1,
		PrevHash:        bytes.Repeat([]byte{0}, 32),
		Signature:       bytes.Repeat([]byte{0xab}, 64),
	}
	entry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: rating}}
	ok, err := ledger.VerifyEntry(entry)
	if ok {
		t.Fatal("VerifyEntry accepted entry with invalid RaterPeerID")
	}
	if err == nil {
		t.Fatal("expected error for invalid RaterPeerID, got nil")
	}
	// Sanity: surfaces as an inability to parse the peer id.
	if !errors.Is(err, ledger.ErrInvalidRaterPeerID) {
		t.Fatalf("want ErrInvalidRaterPeerID, got %v", err)
	}
}

func TestEntryHash_EmptyBody_ReturnsZero(t *testing.T) {
	// Calling EntryHash on a malformed entry shouldn't panic. The
	// contract is that the returned hash is meaningless on bad input,
	// but the function returns rather than crashing the process — any
	// upstream caller treats this as a verification failure.
	var zero [32]byte
	if got := ledger.EntryHash(&pb.SignedEntry{}); got != zero {
		t.Fatalf("EntryHash on empty body returned %x, want zero", got)
	}
}

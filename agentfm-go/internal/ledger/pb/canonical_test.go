package pb_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	pb "agentfm/internal/ledger/pb"

	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// fixtures
//
// Each newX helper returns a fully-populated envelope of that type with no
// shared mutable state between callers, so tests can freely mutate the
// returned value without affecting other tests.
// -----------------------------------------------------------------------------

func newRating() *pb.Rating {
	return &pb.Rating{
		RaterPeerId:     []byte{0x01, 0x02, 0x03},
		SubjectPeerId:   []byte{0xaa, 0xbb, 0xcc},
		Dimension:       "honesty",
		Score:           0.42,
		Context:         "task_abc",
		TimestampUnixNs: 1_700_000_000_000_000_000,
		PrevHash:        bytes.Repeat([]byte{0x11}, 32),
		Signature:       []byte("placeholder-sig"),
	}
}

func newComment() *pb.Comment {
	return &pb.Comment{
		RaterPeerId:     []byte{0x01, 0x02, 0x03},
		SubjectPeerId:   []byte{0xaa, 0xbb, 0xcc},
		TextCid:         bytes.Repeat([]byte{0x55}, 34), // 2-byte multihash prefix + 32-byte digest
		Language:        "en",
		AttachedRating:  bytes.Repeat([]byte{0x33}, 32),
		TimestampUnixNs: 1_700_000_000_000_000_000,
		PrevHash:        bytes.Repeat([]byte{0x22}, 32),
		Signature:       []byte("placeholder-sig"),
	}
}

func newLogHead() *pb.LogHead {
	return &pb.LogHead{
		PeerId:          []byte{0x01, 0x02, 0x03},
		TreeSize:        1024,
		RootHash:        bytes.Repeat([]byte{0x77}, 32),
		TimestampUnixNs: 1_700_000_000_000_000_000,
		Signature:       []byte("peer-own-sig"),
		WitnessSigs: []*pb.WitnessSig{
			{WitnessPeerId: []byte{0x10}, Signature: []byte("w1")},
			{WitnessPeerId: []byte{0x20}, Signature: []byte("w2")},
		},
		RekorAnchor: "uuid-not-empty",
	}
}

func newEquivocationAlert() *pb.EquivocationAlert {
	return &pb.EquivocationAlert{
		PeerId:           []byte{0xde, 0xad},
		HeadA:            newLogHead(),
		HeadB:            newLogHead(),
		WitnessPeerId:    []byte{0xbe, 0xef},
		WitnessSignature: []byte("witness-placeholder"),
		TimestampUnixNs:  1_700_000_000_000_000_000,
	}
}

// -----------------------------------------------------------------------------
// round-trip tests: marshal -> unmarshal -> assert equality
// -----------------------------------------------------------------------------

func TestRoundTrip_Rating(t *testing.T) {
	original := newRating()
	bs, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := &pb.Rating{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(original, got) {
		t.Fatalf("round-trip mismatch:\n want %+v\n  got %+v", original, got)
	}
}

func TestRoundTrip_Comment(t *testing.T) {
	original := newComment()
	bs, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := &pb.Comment{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(original, got) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestRoundTrip_LogHead(t *testing.T) {
	original := newLogHead()
	bs, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := &pb.LogHead{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(original, got) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestRoundTrip_EquivocationAlert(t *testing.T) {
	original := newEquivocationAlert()
	bs, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := &pb.EquivocationAlert{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(original, got) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestRoundTrip_InclusionProof_BothSignedEntryVariants(t *testing.T) {
	// Inclusion proof with Rating inside the SignedEntry wrapper.
	withRating := &pb.InclusionProof{
		Entry:     &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRating()}},
		Position:  17,
		AuditPath: [][]byte{bytes.Repeat([]byte{0x01}, 32), bytes.Repeat([]byte{0x02}, 32)},
		LogHead:   newLogHead(),
	}
	bs, err := proto.Marshal(withRating)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := &pb.InclusionProof{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(withRating, got) {
		t.Fatalf("round-trip mismatch (rating variant)")
	}

	// Inclusion proof with Comment inside the SignedEntry wrapper.
	withComment := &pb.InclusionProof{
		Entry:    &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: newComment()}},
		Position: 33,
		LogHead:  newLogHead(),
	}
	bs, err = proto.Marshal(withComment)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = &pb.InclusionProof{}
	if err := proto.Unmarshal(bs, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !proto.Equal(withComment, got) {
		t.Fatalf("round-trip mismatch (comment variant)")
	}
}

func TestRoundTrip_SignedEntry_BothOneofVariants(t *testing.T) {
	cases := []struct {
		name  string
		entry *pb.SignedEntry
	}{
		{"rating", &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: newRating()}}},
		{"comment", &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: newComment()}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bs, err := proto.Marshal(tc.entry)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := &pb.SignedEntry{}
			if err := proto.Unmarshal(bs, got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !proto.Equal(tc.entry, got) {
				t.Fatalf("round-trip mismatch")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// canonical-marshal tests
// -----------------------------------------------------------------------------

func TestCanonical_StripsSignatureFields(t *testing.T) {
	// Rating: two entries identical except for Signature must produce the
	// same canonical bytes.
	a := newRating()
	b := newRating()
	b.Signature = []byte("totally-different-sig")

	bytesA, err := pb.CanonicalRating(a)
	if err != nil {
		t.Fatalf("CanonicalRating(a): %v", err)
	}
	bytesB, err := pb.CanonicalRating(b)
	if err != nil {
		t.Fatalf("CanonicalRating(b): %v", err)
	}
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatalf("canonical bytes differ despite only Signature differing")
	}
}

func TestCanonical_LogHead_StripsAllMutableFields(t *testing.T) {
	a := newLogHead()
	b := newLogHead()
	// Mutate every field the canonical helper is supposed to strip.
	b.Signature = []byte("different-own-sig")
	b.WitnessSigs = []*pb.WitnessSig{{WitnessPeerId: []byte{0xff}, Signature: []byte("ghost")}}
	b.RekorAnchor = "different-uuid"

	bytesA, err := pb.CanonicalLogHead(a)
	if err != nil {
		t.Fatalf("CanonicalLogHead(a): %v", err)
	}
	bytesB, err := pb.CanonicalLogHead(b)
	if err != nil {
		t.Fatalf("CanonicalLogHead(b): %v", err)
	}
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatalf("canonical LogHead bytes differ despite only mutable fields changing")
	}
}

func TestCanonical_LogHead_DetectsContentChange(t *testing.T) {
	a := newLogHead()
	b := newLogHead()
	b.TreeSize = a.TreeSize + 1 // content change — MUST be detected

	bytesA, _ := pb.CanonicalLogHead(a)
	bytesB, _ := pb.CanonicalLogHead(b)
	if bytes.Equal(bytesA, bytesB) {
		t.Fatalf("canonical bytes did not change after a content field changed")
	}
}

func TestCanonical_EquivocationAlert_StripsWitnessSignature(t *testing.T) {
	a := newEquivocationAlert()
	b := newEquivocationAlert()
	b.WitnessSignature = []byte("entirely-different")

	bytesA, _ := pb.CanonicalEquivocationAlert(a)
	bytesB, _ := pb.CanonicalEquivocationAlert(b)
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatalf("canonical alert bytes differ despite only witness_signature changing")
	}
}

func TestCanonical_Comment_StripsSignature(t *testing.T) {
	a := newComment()
	b := newComment()
	b.Signature = []byte("nope")

	bytesA, _ := pb.CanonicalComment(a)
	bytesB, _ := pb.CanonicalComment(b)
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatalf("canonical comment bytes differ despite only signature changing")
	}
}

// Property test: calling CanonicalX twice on the same logical message
// produces identical bytes. This is what the Deterministic: true option
// gives us; the test exists to catch regressions if anyone reaches for
// proto.Marshal directly inside canonical.go.
func TestCanonical_Deterministic_AcrossCalls(t *testing.T) {
	rating := newRating()
	first, err := pb.CanonicalRating(rating)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := pb.CanonicalRating(rating)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("call %d produced different bytes; non-deterministic marshal", i)
		}
	}
}

// Property test: CanonicalX does not mutate its input. Critical because
// callers will sign-then-attach-signature — if CanonicalRating cleared the
// caller's Signature in place, the second call (verify) would fail.
func TestCanonical_DoesNotMutateInput(t *testing.T) {
	r := newRating()
	sigBefore := append([]byte(nil), r.Signature...)
	_, _ = pb.CanonicalRating(r)
	if !bytes.Equal(r.Signature, sigBefore) {
		t.Fatalf("CanonicalRating mutated caller's Signature field")
	}

	h := newLogHead()
	witnessCountBefore := len(h.WitnessSigs)
	rekorBefore := h.RekorAnchor
	_, _ = pb.CanonicalLogHead(h)
	if len(h.WitnessSigs) != witnessCountBefore {
		t.Fatalf("CanonicalLogHead mutated caller's WitnessSigs")
	}
	if h.RekorAnchor != rekorBefore {
		t.Fatalf("CanonicalLogHead mutated caller's RekorAnchor")
	}
}

// -----------------------------------------------------------------------------
// tamper detection: flip a content byte, assert SHA-256 of canonical bytes
// changes. This is the property the entire signed-ledger design rests on.
// -----------------------------------------------------------------------------

func TestTamper_Rating_HashChangesOnPayloadFlip(t *testing.T) {
	r := newRating()
	bytesBefore, _ := pb.CanonicalRating(r)
	hashBefore := sha256.Sum256(bytesBefore)

	// Flip the dimension field — a content field, not a signature field.
	r.Dimension = "latency"

	bytesAfter, _ := pb.CanonicalRating(r)
	hashAfter := sha256.Sum256(bytesAfter)

	if hashBefore == hashAfter {
		t.Fatalf("hash unchanged after flipping Rating.Dimension; tamper detection broken")
	}
}

func TestTamper_LogHead_HashChangesOnRootFlip(t *testing.T) {
	h := newLogHead()
	bytesBefore, _ := pb.CanonicalLogHead(h)
	hashBefore := sha256.Sum256(bytesBefore)

	// Flip one bit of the root hash.
	h.RootHash[0] ^= 0x01

	bytesAfter, _ := pb.CanonicalLogHead(h)
	hashAfter := sha256.Sum256(bytesAfter)

	if hashBefore == hashAfter {
		t.Fatalf("hash unchanged after flipping LogHead.RootHash bit; tamper detection broken")
	}
}

// -----------------------------------------------------------------------------
// SignedEntry oneof dispatch
// -----------------------------------------------------------------------------

func TestCanonicalSignedEntry_DispatchesOnBodyVariant(t *testing.T) {
	rating := newRating()
	bytesViaInner, _ := pb.CanonicalRating(rating)
	bytesViaOuter, err := pb.CanonicalSignedEntry(&pb.SignedEntry{
		Body: &pb.SignedEntry_Rating{Rating: rating},
	})
	if err != nil {
		t.Fatalf("CanonicalSignedEntry: %v", err)
	}
	if !bytes.Equal(bytesViaInner, bytesViaOuter) {
		t.Fatalf("SignedEntry(Rating) canonical bytes differ from direct CanonicalRating bytes")
	}

	comment := newComment()
	bytesViaInner, _ = pb.CanonicalComment(comment)
	bytesViaOuter, err = pb.CanonicalSignedEntry(&pb.SignedEntry{
		Body: &pb.SignedEntry_Comment{Comment: comment},
	})
	if err != nil {
		t.Fatalf("CanonicalSignedEntry(comment): %v", err)
	}
	if !bytes.Equal(bytesViaInner, bytesViaOuter) {
		t.Fatalf("SignedEntry(Comment) canonical bytes differ from direct CanonicalComment bytes")
	}
}

func TestCanonicalSignedEntry_EmptyBodyReturnsError(t *testing.T) {
	_, err := pb.CanonicalSignedEntry(&pb.SignedEntry{})
	if err == nil {
		t.Fatalf("expected error for SignedEntry with no body, got nil")
	}
}

func TestCanonical_NilInputsReturnError(t *testing.T) {
	cases := []struct {
		name string
		call func() ([]byte, error)
	}{
		{"Rating", func() ([]byte, error) { return pb.CanonicalRating(nil) }},
		{"Comment", func() ([]byte, error) { return pb.CanonicalComment(nil) }},
		{"LogHead", func() ([]byte, error) { return pb.CanonicalLogHead(nil) }},
		{"EquivocationAlert", func() ([]byte, error) { return pb.CanonicalEquivocationAlert(nil) }},
		{"SignedEntry", func() ([]byte, error) { return pb.CanonicalSignedEntry(nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call()
			if err == nil {
				t.Fatalf("expected error on nil input, got nil")
			}
		})
	}
}

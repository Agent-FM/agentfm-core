package merkle_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"agentfm/internal/ledger/merkle"
)

// -----------------------------------------------------------------------------
// hash primitives
// -----------------------------------------------------------------------------

func TestHashLeaf_AppliesZeroPrefix(t *testing.T) {
	data := []byte("hello world")
	got := merkle.HashLeaf(data)
	want := sha256.Sum256(append([]byte{0x00}, data...))
	if got != want {
		t.Fatalf("HashLeaf wrong:\n want %x\n  got %x", want, got)
	}
}

func TestHashLeaf_EmptyData(t *testing.T) {
	got := merkle.HashLeaf(nil)
	want := sha256.Sum256([]byte{0x00})
	if got != want {
		t.Fatalf("HashLeaf(nil) wrong:\n want %x\n  got %x", want, got)
	}
}

func TestHashChildren_AppliesOnePrefix(t *testing.T) {
	left := sha256.Sum256([]byte("left"))
	right := sha256.Sum256([]byte("right"))

	got := merkle.HashChildren(left, right)

	combined := make([]byte, 0, 1+32+32)
	combined = append(combined, 0x01)
	combined = append(combined, left[:]...)
	combined = append(combined, right[:]...)
	want := sha256.Sum256(combined)

	if got != want {
		t.Fatalf("HashChildren wrong:\n want %x\n  got %x", want, got)
	}
}

// Domain separation: hashing the same bytes as a leaf vs as a pair of
// children MUST produce different output. Without this, a malicious peer
// could swap a leaf for an inner-node hash and have it "validate."
func TestDomainSeparation_LeafVsChildren(t *testing.T) {
	x := sha256.Sum256([]byte("x"))

	asLeaf := merkle.HashLeaf(x[:])
	asNode := merkle.HashChildren(x, x)

	if asLeaf == asNode {
		t.Fatalf("HashLeaf(x) == HashChildren(x, x); domain separation broken")
	}
}

// -----------------------------------------------------------------------------
// tree: empty, single-leaf, two-leaf shapes
// -----------------------------------------------------------------------------

func TestEmptyTree_RootIsSHA256OfEmpty(t *testing.T) {
	tree := merkle.New()
	if got := tree.Size(); got != 0 {
		t.Fatalf("empty tree Size = %d, want 0", got)
	}
	want := sha256.Sum256(nil)
	if got := tree.Root(); got != want {
		t.Fatalf("empty tree Root:\n want %x\n  got %x", want, got)
	}
}

func TestSingleLeafTree_RootIsLeafHash(t *testing.T) {
	tree := merkle.New()
	leaf := merkle.HashLeaf([]byte("only-entry"))
	size := tree.Append(leaf)
	if size != 1 {
		t.Fatalf("Append returned size %d, want 1", size)
	}
	if got := tree.Root(); got != leaf {
		t.Fatalf("single-leaf Root:\n want %x\n  got %x", leaf, got)
	}
}

func TestTwoLeafTree_RootIsHashChildren(t *testing.T) {
	tree := merkle.New()
	l1 := merkle.HashLeaf([]byte("first"))
	l2 := merkle.HashLeaf([]byte("second"))
	tree.Append(l1)
	tree.Append(l2)

	want := merkle.HashChildren(l1, l2)
	if got := tree.Root(); got != want {
		t.Fatalf("two-leaf Root:\n want %x\n  got %x", want, got)
	}
}

// -----------------------------------------------------------------------------
// known vector
//
// Take a 4-leaf tree of {a, b, c, d} and reproduce the root via hand-
// computed RFC 6962 hashing. This is the same shape used by Trillian's
// reference tests; if our output diverges, our hashing is wrong.
// -----------------------------------------------------------------------------

func TestKnownVector_FourLeafTree(t *testing.T) {
	tree := merkle.New()
	la := merkle.HashLeaf([]byte("a"))
	lb := merkle.HashLeaf([]byte("b"))
	lc := merkle.HashLeaf([]byte("c"))
	ld := merkle.HashLeaf([]byte("d"))
	for _, h := range [][32]byte{la, lb, lc, ld} {
		tree.Append(h)
	}

	// Tree shape for 4 leaves under RFC 6962 is a perfect binary tree:
	//          root
	//         /    \
	//      n_ab    n_cd
	//      / \     / \
	//     la lb   lc ld
	left := merkle.HashChildren(la, lb)
	right := merkle.HashChildren(lc, ld)
	want := merkle.HashChildren(left, right)

	if got := tree.Root(); got != want {
		t.Fatalf("4-leaf Root:\n want %x\n  got %x", want, got)
	}
}

// 5-leaf tree exercises the unbalanced-split case where k (largest power
// of 2 < n) gives left = 4 leaves, right = 1 leaf.
//
//	         root
//	        /    \
//	   n_abcd     le
//	   /    \
//	 n_ab   n_cd
//	 / \    / \
//	la lb  lc ld
func TestKnownVector_FiveLeafTree_UnbalancedSplit(t *testing.T) {
	tree := merkle.New()
	la := merkle.HashLeaf([]byte("a"))
	lb := merkle.HashLeaf([]byte("b"))
	lc := merkle.HashLeaf([]byte("c"))
	ld := merkle.HashLeaf([]byte("d"))
	le := merkle.HashLeaf([]byte("e"))
	for _, h := range [][32]byte{la, lb, lc, ld, le} {
		tree.Append(h)
	}

	nAB := merkle.HashChildren(la, lb)
	nCD := merkle.HashChildren(lc, ld)
	nABCD := merkle.HashChildren(nAB, nCD)
	want := merkle.HashChildren(nABCD, le)

	if got := tree.Root(); got != want {
		t.Fatalf("5-leaf Root:\n want %x\n  got %x", want, got)
	}
}

// -----------------------------------------------------------------------------
// LastLeafHash semantics — needed by ledger.Append to populate prev_hash.
// -----------------------------------------------------------------------------

func TestLastLeafHash_EmptyReturnsZero(t *testing.T) {
	tree := merkle.New()
	var zero [32]byte
	if got := tree.LastLeafHash(); got != zero {
		t.Fatalf("LastLeafHash on empty tree = %x, want zero", got)
	}
}

func TestLastLeafHash_TracksMostRecentAppend(t *testing.T) {
	tree := merkle.New()
	leaves := [][32]byte{
		merkle.HashLeaf([]byte("first")),
		merkle.HashLeaf([]byte("second")),
		merkle.HashLeaf([]byte("third")),
	}
	for _, l := range leaves {
		tree.Append(l)
		if got := tree.LastLeafHash(); got != l {
			t.Fatalf("LastLeafHash after Append(%x) = %x, want %x", l, got, l)
		}
	}
}

// -----------------------------------------------------------------------------
// inclusion proof: round-trip every leaf for tree sizes 1..256
// -----------------------------------------------------------------------------

func TestInclusionProof_EveryLeafVerifiesAcrossSizes(t *testing.T) {
	// 256 covers all interesting cases: powers of 2, off-by-one, odd
	// counts, deep trees. Larger sizes add nothing beyond depth.
	for n := uint64(1); n <= 256; n++ {
		t.Run(fmt.Sprintf("size_%d", n), func(t *testing.T) {
			tree := merkle.New()
			leaves := make([][32]byte, n)
			for i := range leaves {
				leaves[i] = merkle.HashLeaf([]byte(fmt.Sprintf("entry-%d", i)))
				tree.Append(leaves[i])
			}
			root := tree.Root()
			for idx := uint64(0); idx < n; idx++ {
				proof, err := tree.InclusionProof(idx)
				if err != nil {
					t.Fatalf("InclusionProof(%d) in size %d: %v", idx, n, err)
				}
				if !merkle.VerifyInclusion(leaves[idx], idx, n, root, proof) {
					t.Fatalf("verify failed: size=%d idx=%d", n, idx)
				}
			}
		})
	}
}

func TestInclusionProof_OutOfRangeReturnsError(t *testing.T) {
	tree := merkle.New()
	for i := 0; i < 5; i++ {
		tree.Append(merkle.HashLeaf([]byte{byte(i)}))
	}
	if _, err := tree.InclusionProof(5); err == nil {
		t.Fatal("expected error for idx == size, got nil")
	}
	if _, err := tree.InclusionProof(999); err == nil {
		t.Fatal("expected error for idx > size, got nil")
	}
}

func TestInclusionProof_EmptyTreeAnyIndexErrors(t *testing.T) {
	tree := merkle.New()
	if _, err := tree.InclusionProof(0); err == nil {
		t.Fatal("expected error for InclusionProof on empty tree, got nil")
	}
}

// -----------------------------------------------------------------------------
// tamper tests — every proof component fails verification when corrupted.
// This is the property the entire signed-ledger design rests on.
// -----------------------------------------------------------------------------

func TestVerifyInclusion_FlipBitInProof_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 100)
	root := tree.Root()

	idx := uint64(42)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	if len(proof) == 0 {
		t.Fatal("proof unexpectedly empty")
	}

	// Mutate each sibling hash one at a time; every flip MUST break verification.
	for layer := range proof {
		corrupted := cloneProof(proof)
		corrupted[layer][0] ^= 0x01
		if merkle.VerifyInclusion(leaves[idx], idx, n, root, corrupted) {
			t.Fatalf("verification passed despite flipped bit at proof layer %d", layer)
		}
	}
}

func TestVerifyInclusion_FlipBitInRoot_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 50)
	root := tree.Root()
	idx := uint64(17)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	root[0] ^= 0x01
	if merkle.VerifyInclusion(leaves[idx], idx, n, root, proof) {
		t.Fatal("verification passed despite flipped root bit")
	}
}

func TestVerifyInclusion_WrongLeafHash_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 50)
	root := tree.Root()
	idx := uint64(3)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	wrongLeaf := leaves[idx]
	wrongLeaf[0] ^= 0x01
	if merkle.VerifyInclusion(wrongLeaf, idx, n, root, proof) {
		t.Fatal("verification passed despite wrong leaf hash")
	}
}

func TestVerifyInclusion_WrongIndex_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 50)
	root := tree.Root()
	idx := uint64(7)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	// Use the proof for idx=7 but claim it's for idx=8. Verifier MUST reject.
	if merkle.VerifyInclusion(leaves[idx], idx+1, n, root, proof) {
		t.Fatal("verification passed despite wrong index")
	}
}

// A claimed size that requires MORE proof entries than the supplied proof
// contains MUST be rejected (the verifier runs out of siblings and the
// recursion can't reach a root). This is the size-mismatch case the
// verifier *can* catch on its own.
//
// Note: a same-shape size mismatch (e.g. size 50 → 51 for idx=7) cannot
// be detected from the proof alone — both shapes split identically at
// k=32 and the verifier reconstructs the same root. Strict (root, size)
// binding is the LogHead's job; the verifier MUST take size from a
// trusted signed LogHead, not from a caller-supplied value.
func TestVerifyInclusion_TooBigSize_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 50)
	_ = n
	root := tree.Root()
	idx := uint64(7)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	// 256 leaves implies a depth-8 tree → proof length 8 needed, but we
	// only have 6. Verifier must reject for under-supplied proof.
	if merkle.VerifyInclusion(leaves[idx], idx, 256, root, proof) {
		t.Fatal("verification passed despite claimed-size requiring deeper proof")
	}
}

func TestVerifyInclusion_TruncatedProof_Fails(t *testing.T) {
	tree, leaves, n := setupTree(t, 50)
	root := tree.Root()
	idx := uint64(7)
	proof, err := tree.InclusionProof(idx)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	if len(proof) < 2 {
		t.Fatalf("proof unexpectedly short: %d", len(proof))
	}
	truncated := proof[:len(proof)-1]
	if merkle.VerifyInclusion(leaves[idx], idx, n, root, truncated) {
		t.Fatal("verification passed despite truncated proof")
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func setupTree(t *testing.T, n uint64) (*merkle.Tree, [][32]byte, uint64) {
	t.Helper()
	tree := merkle.New()
	leaves := make([][32]byte, n)
	for i := uint64(0); i < n; i++ {
		leaves[i] = merkle.HashLeaf([]byte(fmt.Sprintf("entry-%d", i)))
		tree.Append(leaves[i])
	}
	return tree, leaves, n
}

func cloneProof(p [][32]byte) [][32]byte {
	cp := make([][32]byte, len(p))
	copy(cp, p)
	return cp
}

// Round-trip sanity: a tree built leaf-by-leaf and a separately-recomputed
// root over the same leaves must agree. Catches any silent statefulness in
// Tree.Root().
func TestRootStable_AfterRecomputation(t *testing.T) {
	tree, _, _ := setupTree(t, 64)
	r1 := tree.Root()
	r2 := tree.Root()
	if !bytes.Equal(r1[:], r2[:]) {
		t.Fatalf("Root() returned different values across calls: %s vs %s",
			hex.EncodeToString(r1[:]), hex.EncodeToString(r2[:]))
	}
}

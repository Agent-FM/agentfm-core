package merkle_test

import (
	"errors"
	"fmt"
	"testing"

	"agentfm/internal/ledger/merkle"
)

// buildTreeOfSize returns a Tree containing exactly n leaves, plus
// the leaves themselves and the root snapshot taken at that point.
func buildTreeOfSize(t *testing.T, n uint64) (*merkle.Tree, [][32]byte, [32]byte) {
	t.Helper()
	tree := merkle.New()
	leaves := make([][32]byte, n)
	for i := uint64(0); i < n; i++ {
		leaves[i] = merkle.HashLeaf([]byte(fmt.Sprintf("e-%d", i)))
		tree.Append(leaves[i])
	}
	return tree, leaves, tree.Root()
}

func TestConsistencyProof_OldSizeZero_Errors(t *testing.T) {
	tree, _, _ := buildTreeOfSize(t, 5)
	if _, err := tree.ConsistencyProof(0); !errors.Is(err, merkle.ErrConsistencyOutOfRange) {
		t.Fatalf("want ErrConsistencyOutOfRange, got %v", err)
	}
}

func TestConsistencyProof_OldSizeBeyondNew_Errors(t *testing.T) {
	tree, _, _ := buildTreeOfSize(t, 5)
	if _, err := tree.ConsistencyProof(99); !errors.Is(err, merkle.ErrConsistencyOutOfRange) {
		t.Fatalf("want ErrConsistencyOutOfRange, got %v", err)
	}
}

func TestConsistencyProof_OldEqualsNew_EmptyProofAndRootsMustMatch(t *testing.T) {
	tree, _, root := buildTreeOfSize(t, 7)
	proof, err := tree.ConsistencyProof(7)
	if err != nil {
		t.Fatalf("ConsistencyProof: %v", err)
	}
	if len(proof) != 0 {
		t.Fatalf("equal-size consistency proof should be empty, got %d hashes", len(proof))
	}
	if !merkle.VerifyConsistency(7, 7, root, root, proof) {
		t.Fatal("Verify of identical roots / empty proof should pass")
	}
	// Different roots at the same size — must fail.
	bad := root
	bad[0] ^= 0x01
	if merkle.VerifyConsistency(7, 7, root, bad, proof) {
		t.Fatal("Verify must reject when same-size roots differ")
	}
}

// Round-trip every (oldSize, newSize) pair with oldSize <= newSize for
// trees up to 50 leaves. This catches off-by-one bugs in either the
// prover or the verifier.
func TestConsistencyProof_RoundTripEveryPair(t *testing.T) {
	const maxN = 50
	// Pre-compute snapshots: builder yields all (size, root, leaves).
	type snap struct {
		root   [32]byte
		leaves [][32]byte
	}
	snaps := make([]snap, maxN+1)
	for n := uint64(1); n <= maxN; n++ {
		_, leaves, root := buildTreeOfSize(t, n)
		snaps[n] = snap{root: root, leaves: leaves}
	}
	for newSize := uint64(1); newSize <= maxN; newSize++ {
		tree, _, newRoot := buildTreeOfSize(t, newSize)
		for oldSize := uint64(1); oldSize <= newSize; oldSize++ {
			proof, err := tree.ConsistencyProof(oldSize)
			if err != nil {
				t.Fatalf("ConsistencyProof(old=%d, new=%d): %v", oldSize, newSize, err)
			}
			oldRoot := snaps[oldSize].root
			if !merkle.VerifyConsistency(oldSize, newSize, oldRoot, newRoot, proof) {
				t.Fatalf("verify failed: old=%d new=%d", oldSize, newSize)
			}
		}
	}
}

// Negative test: flipping a bit in the proof MUST break verification.
func TestConsistencyProof_TamperedProofFails(t *testing.T) {
	tree, _, newRoot := buildTreeOfSize(t, 17)
	_, _, oldRoot := buildTreeOfSize(t, 7)

	proof, err := tree.ConsistencyProof(7)
	if err != nil {
		t.Fatalf("ConsistencyProof: %v", err)
	}
	if len(proof) == 0 {
		t.Fatal("proof unexpectedly empty")
	}
	for i := range proof {
		corrupted := make([][32]byte, len(proof))
		copy(corrupted, proof)
		corrupted[i][0] ^= 0x01
		if merkle.VerifyConsistency(7, 17, oldRoot, newRoot, corrupted) {
			t.Fatalf("verify accepted tampered proof at index %d", i)
		}
	}
}

// Negative test: wrong oldRoot must be rejected.
func TestConsistencyProof_WrongOldRoot_Rejected(t *testing.T) {
	tree, _, newRoot := buildTreeOfSize(t, 20)
	_, _, oldRoot := buildTreeOfSize(t, 8)

	proof, _ := tree.ConsistencyProof(8)
	wrongOld := oldRoot
	wrongOld[3] ^= 0x10
	if merkle.VerifyConsistency(8, 20, wrongOld, newRoot, proof) {
		t.Fatal("verify accepted wrong oldRoot")
	}
}

// Negative test: wrong newRoot must be rejected.
func TestConsistencyProof_WrongNewRoot_Rejected(t *testing.T) {
	tree, _, newRoot := buildTreeOfSize(t, 20)
	_, _, oldRoot := buildTreeOfSize(t, 8)

	proof, _ := tree.ConsistencyProof(8)
	wrongNew := newRoot
	wrongNew[10] ^= 0xff
	if merkle.VerifyConsistency(8, 20, oldRoot, wrongNew, proof) {
		t.Fatal("verify accepted wrong newRoot")
	}
}

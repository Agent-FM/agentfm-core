package merkle

import (
	"crypto/sha256"
	"fmt"
)

// Tree is an in-memory, append-only RFC 6962 Merkle tree over SHA-256.
//
// Concurrency: Tree is NOT goroutine-safe. The intended caller is the
// single-writer ledger in internal/ledger/ledger.go, which serialises
// Append behind its own mutex. Callers that need concurrent reads
// should snapshot Root + LastLeafHash under the same lock that guards
// Append.
type Tree struct {
	// leaves stores the 32-byte leaf hashes in insertion order. RFC
	// 6962 uses these directly — there is no need to retain raw data.
	leaves [][32]byte
}

// New returns an empty Tree. Its Root is SHA-256(""); its Size is 0;
// LastLeafHash is the zero array.
func New() *Tree {
	return &Tree{}
}

// Append adds a leaf hash to the tree and returns the new tree size
// (i.e. the 1-based count of leaves after this call). The leaf hash
// SHOULD be the output of HashLeaf — passing raw data here will
// produce a tree that disagrees with every other RFC 6962 implementation.
func (t *Tree) Append(leafHash [32]byte) (size uint64) {
	t.leaves = append(t.leaves, leafHash)
	return uint64(len(t.leaves))
}

// Size returns the number of leaves currently in the tree.
func (t *Tree) Size() uint64 {
	return uint64(len(t.leaves))
}

// LastLeafHash returns the most recently appended leaf hash. If the
// tree is empty, returns the zero array — this is the sentinel value
// the ledger's first entry uses as its prev_hash.
func (t *Tree) LastLeafHash() [32]byte {
	if len(t.leaves) == 0 {
		var zero [32]byte
		return zero
	}
	return t.leaves[len(t.leaves)-1]
}

// Root computes the tree's current Merkle root.
//
// Per RFC 6962 §2.1:
//
//	MTH({}) = SHA-256("")
//	MTH({d0}) = HashLeaf(d0)   // already-hashed leaves in our API
//	MTH(D[n]) = HashChildren(
//	             MTH(D[0:k]),
//	             MTH(D[k:n]),
//	           )
//
// where k is the largest power of 2 < n. For now this recomputes from
// the leaf slice each time — O(n log n). P1-2 will add a SQLite-backed
// store that caches intermediate node hashes so Root is O(log n).
func (t *Tree) Root() [32]byte {
	if len(t.leaves) == 0 {
		var out [32]byte
		copy(out[:], sha256.New().Sum(nil))
		return out
	}
	return computeRoot(t.leaves)
}

// computeRoot is the RFC 6962 recursive Merkle Tree Hash (MTH) over the
// supplied slice of already-hashed leaves. Callers must ensure len > 0.
func computeRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	k := largestPowerOfTwoLessThan(uint64(len(leaves)))
	left := computeRoot(leaves[:k])
	right := computeRoot(leaves[k:])
	return HashChildren(left, right)
}

// largestPowerOfTwoLessThan returns the largest 2^x with 2^x < n.
// Per RFC 6962, this is the split point for the left subtree when
// building a Merkle Tree Hash over n leaves (n >= 2).
//
// Examples:
//
//	n=2  -> 1
//	n=3  -> 2
//	n=4  -> 2
//	n=5  -> 4
//	n=8  -> 4
//	n=9  -> 8
func largestPowerOfTwoLessThan(n uint64) uint64 {
	if n < 2 {
		// Programming error — callers (computeRoot, path,
		// rebuildRoot, foldConsistency, consistencyProof) all guard
		// with explicit n==1 or m==n base cases. A silent return
		// would mask a refactor bug; panic instead so the test suite
		// catches it loudly. Fix-12 audit finding.
		panic(fmt.Sprintf("merkle.largestPowerOfTwoLessThan: n=%d (must be >= 2)", n))
	}
	k := uint64(1)
	for k<<1 < n {
		k <<= 1
	}
	return k
}

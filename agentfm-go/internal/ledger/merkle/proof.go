package merkle

import "fmt"

// InclusionProof returns the RFC 6962 audit path for the leaf at idx
// against the tree's current state. The returned slice contains the
// sibling hashes from the leaf level up to (but not including) the
// root, in bottom-up order — exactly the order VerifyInclusion
// consumes them.
//
// Errors:
//
//   - ErrIndexOutOfRange if idx >= Size().
//   - ErrEmptyTree if Size() == 0.
func (t *Tree) InclusionProof(idx uint64) ([][32]byte, error) {
	n := uint64(len(t.leaves))
	if n == 0 {
		return nil, ErrEmptyTree
	}
	if idx >= n {
		return nil, fmt.Errorf("%w: idx=%d size=%d", ErrIndexOutOfRange, idx, n)
	}
	return path(idx, t.leaves), nil
}

// path implements the RFC 6962 §2.1.1 PATH algorithm:
//
//	PATH(m, D[n]) = []                                            // n == 1
//	PATH(m, D[n]) = PATH(m,   D[0:k])  :: MTH(D[k:n])             // m < k
//	PATH(m, D[n]) = PATH(m-k, D[k:n])  :: MTH(D[0:k])             // m >= k
//
// The returned slice grows from leaf-level siblings upward.
func path(m uint64, leaves [][32]byte) [][32]byte {
	n := uint64(len(leaves))
	if n == 1 {
		return nil
	}
	k := largestPowerOfTwoLessThan(n)
	if m < k {
		return append(path(m, leaves[:k]), computeRoot(leaves[k:]))
	}
	return append(path(m-k, leaves[k:]), computeRoot(leaves[:k]))
}

// VerifyInclusion validates that leafHash sits at position idx in a
// tree of size size whose root is root, using the supplied audit
// proof. Standalone (no Tree required) so other peers can verify
// proofs they receive over the network.
//
// The function returns false on any inconsistency: short proof,
// long proof, wrong-shape proof, mismatched root, mismatched leaf,
// index out of range. It MUST NOT panic on bad input — proofs
// arrive over libp2p from peers who may be hostile.
//
// SECURITY BOUNDARY: an RFC 6962 inclusion proof binds (leafHash, idx,
// size, root). Callers MUST pass `size` and `root` from a trusted
// signed LogHead — if the caller's `size` differs from the size the
// proof was generated under but the recursion shape happens to match
// (e.g. size 50 vs 51 for idx 7 both split at k=32), the verifier
// cannot distinguish the two from the proof alone. The signed LogHead
// pairs (root, size) together so this attack is moot in practice; just
// never let an untrusted source override the size.
func VerifyInclusion(leafHash [32]byte, idx uint64, size uint64, root [32]byte, proof [][32]byte) bool {
	if size == 0 || idx >= size {
		return false
	}
	derived, consumed, ok := rebuildRoot(leafHash, idx, size, proof, 0)
	if !ok {
		return false
	}
	// Any unconsumed sibling means the proof was longer than the tree
	// shape allows for this (idx, size) — reject as malformed.
	if consumed != len(proof) {
		return false
	}
	return derived == root
}

// rebuildRoot mirrors the recursive PATH algorithm used to generate the
// proof (see path()): it descends top-down, consumes sibling hashes
// from proof in the same order they were appended, and combines them
// in the correct (left, right) orientation as the recursion unwinds.
//
// Returns the derived root for the (idx, size) window, the number of
// proof entries consumed, and ok=false on under-supplied proof.
func rebuildRoot(leafHash [32]byte, idx uint64, size uint64, proof [][32]byte, cursor int) ([32]byte, int, bool) {
	if size == 1 {
		// Leaf level: no sibling, no hash combine.
		return leafHash, cursor, true
	}
	k := largestPowerOfTwoLessThan(size)
	if idx < k {
		sub, next, ok := rebuildRoot(leafHash, idx, k, proof, cursor)
		if !ok || next >= len(proof) {
			return [32]byte{}, 0, false
		}
		return HashChildren(sub, proof[next]), next + 1, true
	}
	sub, next, ok := rebuildRoot(leafHash, idx-k, size-k, proof, cursor)
	if !ok || next >= len(proof) {
		return [32]byte{}, 0, false
	}
	return HashChildren(proof[next], sub), next + 1, true
}

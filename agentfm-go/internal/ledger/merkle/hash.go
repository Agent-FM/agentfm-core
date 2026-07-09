// Package merkle implements an RFC 6962-style append-only Merkle tree
// over SHA-256.
//
// The package is intentionally minimal: no persistence, no caching, no
// goroutine plumbing. All state is in-memory and owned by a single
// caller. Persistence lands in internal/ledger/store (P1-2); the
// goroutine-safe wrapper lands in internal/ledger/ledger.go (P1-4).
//
// # Hash discipline
//
// RFC 6962 demands domain separation between leaf hashes and internal
// node hashes so that an inner-node hash can never be mistaken for a
// leaf hash (and vice versa):
//
//	leaf_hash   = SHA-256(0x00 || data)
//	node_hash   = SHA-256(0x01 || left_child_hash || right_child_hash)
//	empty_tree  = SHA-256("")
//
// These constants are exposed as HashLeaf / HashChildren so other
// packages (e.g. internal/ledger/sign.go in P1-3) can compute the leaf
// hash of an entry before handing it to Tree.Append.
package merkle

import "crypto/sha256"

const (
	// leafPrefix is prepended to user data before hashing to form a leaf
	// hash. Per RFC 6962 §2.1.
	leafPrefix byte = 0x00

	// nodePrefix is prepended to a left||right concatenation before
	// hashing to form an internal node hash. Per RFC 6962 §2.1.
	nodePrefix byte = 0x01
)

// HashLeaf returns the RFC 6962 leaf hash for data: SHA-256(0x00 || data).
//
// Callers should treat this as the canonical, persistent identity of an
// entry — it is what gets stored in the ledger, what other entries
// reference as prev_hash, and what gets passed to Tree.Append.
func HashLeaf(data []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{leafPrefix})
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// HashChildren returns the RFC 6962 internal-node hash:
// SHA-256(0x01 || left || right).
//
// Both children MUST already be 32-byte hashes (either leaf hashes from
// HashLeaf or recursive node hashes from a prior HashChildren call).
// No mixing of raw data into a node hash is permitted — that would
// break domain separation.
func HashChildren(left, right [32]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{nodePrefix})
	h.Write(left[:])
	h.Write(right[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

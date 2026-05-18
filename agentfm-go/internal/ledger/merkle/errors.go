package merkle

import "errors"

// ErrEmptyTree is returned by InclusionProof when called on a tree
// with zero leaves — there is no leaf at any index to prove.
var ErrEmptyTree = errors.New("merkle: empty tree has no inclusion proofs")

// ErrIndexOutOfRange is returned by InclusionProof when idx >= Size().
// Wrap with %w when surfacing to callers so they can errors.Is-check.
var ErrIndexOutOfRange = errors.New("merkle: leaf index out of range")

// ErrConsistencyOutOfRange is returned by ConsistencyProof when the
// requested oldSize is zero or exceeds the current tree's size.
var ErrConsistencyOutOfRange = errors.New("merkle: consistency old size out of range")

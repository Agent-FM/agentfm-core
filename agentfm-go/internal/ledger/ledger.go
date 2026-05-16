package ledger

import (
	"context"

	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// Entry is the materialised view of a SignedEntry once it has been
// written to (or read from) a ledger. Callers receive Entry values from
// fetch / iterate APIs added in P1-2.
//
//   - Hash is SHA-256(CanonicalSignedEntry(Payload)) — what the Merkle
//     tree uses as its leaf hash and what other entries reference as
//     prev_hash.
//   - Index is the zero-based position in the issuing peer's log.
//   - Payload is the verified envelope; never nil for a value returned
//     from a Ledger method.
type Entry struct {
	Hash    [32]byte
	Index   uint64
	Payload *pb.SignedEntry
}

// Ledger is the per-peer signed append-only log. Implementations are
// expected to be goroutine-safe under a single-writer / many-readers
// access pattern — the AgentFM process creates exactly one Ledger per
// libp2p peer at boot and shares it across boss, worker, and api code.
type Ledger interface {
	// Append signs the payload with this ledger's private key, persists
	// it as the next entry in the log, updates the Merkle tree, signs a
	// new LogHead, and publishes the entry on the GossipSub topic
	// agentfm-feedback-v1. Returns the SHA-256 of the canonical bytes of
	// the inserted entry. Best-effort gossip — local persistence success
	// is reported even if publishing fails.
	Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error)

	// Head returns the most recent signed LogHead, including any witness
	// signatures gathered so far. Returns ErrNotImplemented until P1-4.
	Head(ctx context.Context) (*pb.LogHead, error)

	// Prove builds an inclusion proof for an entry identified by its
	// canonical-bytes hash, anchored against the current Head. Returns
	// ErrNotImplemented until P2-5.
	Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error)

	// VerifyEntry validates a SignedEntry received via gossip or pull.
	// It checks the Ed25519 signature against the rater's libp2p key,
	// verifies prev_hash extends the rater's known chain in the local
	// inbox, and either inserts the entry or queues it as an orphan
	// pending its parent. knownHead, if non-nil, supplies the expected
	// LogHead state for stricter verification (used by inclusion-proof
	// validation in P2-5). Returns ErrNotImplemented until P1-5.
	VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error

	// Close flushes any pending state and releases the underlying
	// SQLite handle and gossip subscriptions. Safe to call multiple
	// times; only the first call has effect.
	Close() error
}

// New constructs a Ledger backed by a SQLite database at path, signing
// every appended entry with key. The key MUST be the libp2p Ed25519
// private key whose public half is embedded in this node's PeerID —
// otherwise verifiers on other peers will reject every entry this
// ledger emits.
//
// Until P1-1..P1-4 land, New returns (nil, ErrNotImplemented). Callers
// SHOULD still construct one at boot so the unwired state is detected
// immediately rather than at first Append.
func New(path string, key crypto.PrivKey) (Ledger, error) {
	_ = path
	_ = key
	return nil, ErrNotImplemented
}

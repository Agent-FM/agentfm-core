package ledger

import (
	"context"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
)

// Options bundles the optional dependencies for constructing a Ledger.
type Options struct {
	// Host is the libp2p host used to register the LedgerFetchProtocol
	// and HeadFetchProtocol stream handlers (P2-5, P5-1). nil disables
	// both handlers (local-only / test mode).
	Host host.Host
}

// IsHeadValid reports whether head carries at least threshold
// witness signatures. Callers that consume reputation (P3-7) should
// gate on this to ignore "advisory" heads — heads with too few
// witness sigs to be considered confirmed by the local trust
// configuration.
//
// Threshold 0 always returns true (no quorum required).
//
// CAVEAT (Fix-9 audit finding): this function does NOT verify the
// signatures themselves — it only COUNTS them. A peer with control
// of its own LogHead bytes can pad WitnessSigs with garbage to
// clear the threshold. For strict admission control, callers should
// additionally validate each WitnessSig against the witness's
// expected PeerID + the canonical LogHead bytes via
// pb.CanonicalLogHead + crypto.PubKey.Verify. Sig-counting is the
// fast path for routing decisions where the cost of full Verify per
// dispatch would be prohibitive; do the full verification on the
// audit / disputed path.
func IsHeadValid(head *pb.LogHead, threshold int) bool {
	if head == nil {
		return false
	}
	if threshold <= 0 {
		return true
	}
	return len(head.WitnessSigs) >= threshold
}

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

	// InboxHas reports whether an entry (raterID, entryHash) has been
	// ingested into the local inbox from gossip (or via VerifyEntry).
	// Used by P2-5 inclusion-proof handling and by tests that need to
	// observe gossip-driven ingestion deterministically.
	InboxHas(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error)

	// IsEquivocator reports whether peerID has been marked as a
	// permanent equivocator by some witness alert this node has
	// processed. P3-7 uses this to floor the peer's reputation score
	// at -1.0 regardless of any positive ratings.
	IsEquivocator(ctx context.Context, peerID []byte) (bool, error)

	// AcceptEntry decodes payload as a pb.SignedEntry proto and routes
	// it through the inbox accept/orphan/promote path — equivalent to
	// VerifyEntry(ctx, entry, nil) but accepts raw proto bytes so
	// callers that obtained the bytes from a network fetch (e.g.
	// CatchUp) do not need to unmarshal themselves. Returns the same
	// typed errors as VerifyEntry.
	AcceptEntry(ctx context.Context, payload []byte) error

	// LastInboxIdx returns the idx of the last relay entry the boss
	// has already ingested so CatchUp can start paginating from
	// lastIdx+1 instead of from 1. Returns 0 on a fresh / empty
	// inbox (catch-up then starts from 1). The idx space is the
	// relay's own entries table sequence, not the inbox's internal
	// ordering — callers MUST treat this as a best-effort hint; the
	// inbox deduplicates so starting from a lower idx is always safe.
	LastInboxIdx(ctx context.Context) (uint64, error)

	// Store returns the underlying SQLite store for direct access by
	// test helpers and the reputation engine's read-only walks. Production
	// code should prefer the higher-level Ledger methods.
	Store() *store.Store

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
// ps is the GossipSub instance the ledger publishes appended entries
// on (topic network.FeedbackTopic). Pass nil to run in local-only
// mode: writes still persist and pass through the Merkle tree, but
// nothing is disseminated. Production bootstrap always supplies a
// real *pubsub.PubSub; tests that don't care about gossip may omit it.
//
// The implementation rebuilds the in-memory Merkle tree from the
// on-disk store at Open, so a process restart resumes the chain at
// the correct prev_hash without losing any entries.
func New(path string, key crypto.PrivKey, ps *pubsub.PubSub) (Ledger, error) {
	return newImpl(path, key, ps, Options{})
}

// NewWithOptions is the extended constructor for callers that need to
// supply a libp2p host for fetch protocol handlers. Otherwise identical to New.
func NewWithOptions(path string, key crypto.PrivKey, ps *pubsub.PubSub, opts Options) (Ledger, error) {
	return newImpl(path, key, ps, opts)
}

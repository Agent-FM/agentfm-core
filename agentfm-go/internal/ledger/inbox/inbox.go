// Package inbox owns the per-peer accept/orphan/promote logic for
// gossiped ledger entries (P1-5).
//
// The package is structurally separate from the local Ledger so that
// the failure modes of receiving untrusted entries (bad signature,
// out-of-order chain, orphan flooding) cannot accidentally corrupt
// the local peer's own append-only log.
//
// Flow on AcceptOrQueue:
//
//  1. Drop self-messages (entry's RaterPeerID == this node's ID).
//  2. Verify the Ed25519 signature against the embedded RaterPeerID.
//     Bad sig → ErrSignatureInvalid; entry is silently dropped.
//  3. Dedupe against inbox_entries and inbox_orphans. Already-seen → nil.
//  4. Chain-extension check:
//     - prev_hash is 32 zero bytes AND no chain head known → first
//     entry from this peer, accept it.
//     - prev_hash equals the rater's known chain head → accept,
//     then promote any orphans waiting on this entry's hash.
//     - otherwise → queue as orphan (subject to OrphanCap).
package inbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// Verifier reports whether entry's signature is valid. Injected at
// construction (rather than imported) to keep the inbox package
// independent of the parent ledger package — otherwise we'd have an
// import cycle (ledger imports inbox to use it, inbox would import
// ledger to call VerifyEntry).
type Verifier func(entry *pb.SignedEntry) (bool, error)

// Hasher returns the canonical leaf hash (merkle.HashLeaf of canonical
// bytes) of an entry. Injected for the same reason as Verifier.
type Hasher func(entry *pb.SignedEntry) [32]byte

// DefaultOrphanCap is the total number of orphan entries the inbox
// will hold across all peers before it starts rejecting new ones.
// 10k orphans × ~1 KiB each = ~10 MiB worst-case footprint, generous
// for adversarial scenarios while still bounding memory.
const DefaultOrphanCap = 10_000

// DefaultPerPeerOrphanCap bounds how many orphans a SINGLE peer may hold.
// Without it one sybil rater could fill the whole global cap and drop
// every honest peer's out-of-order entry. Capped below the global cap so
// the table stays shared across many peers.
const DefaultPerPeerOrphanCap = 1_000

// ErrSignatureInvalid is returned by AcceptOrQueue when the entry's
// Ed25519 signature does not verify against its RaterPeerID. Surfaced
// for tests / logging; the gossip subscriber silently drops it.
var ErrSignatureInvalid = errors.New("inbox: signature invalid")

// ErrSelfMessage is returned by AcceptOrQueue when the entry's
// RaterPeerID equals this node's own peer ID. Receivers MUST NOT
// re-ingest their own outgoing entries via the inbox.
var ErrSelfMessage = errors.New("inbox: entry is self-authored, ignored")

// ErrOrphanCapExceeded is returned when the orphan queue is full and
// a new orphan cannot be accepted. Callers should log + drop.
var ErrOrphanCapExceeded = errors.New("inbox: orphan cap exceeded")

// ErrInvalidEntry is returned for entries whose body is unset or
// whose RaterPeerID cannot be parsed. Programming / wire-format
// errors; not adversarial in the cryptographic sense.
var ErrInvalidEntry = errors.New("inbox: invalid entry shape")

// ErrInvalidPayload is returned when an entry's payload fields are
// out of the bounds declared in proto/agentfm/ledger/v1/ledger.proto
// (e.g. score outside [-1, +1], empty dimension, wrong-length
// prev_hash, non-positive timestamp). Fix-3 audit finding —
// receivers MUST reject these per the proto contract.
var ErrInvalidPayload = errors.New("inbox: invalid payload")

// Inbox coordinates accept / queue / promote for one local peer.
// Goroutine-safe: all writes go through Store's mutex; reads scale.
type Inbox struct {
	store            *store.Store
	ownPeerID        peer.ID
	orphanCap        uint64
	perPeerOrphanCap uint64
	verify           Verifier
	hash             Hasher
}

// New constructs an Inbox bound to a store and an owner peer ID.
// orphanCap of 0 falls back to DefaultOrphanCap. verify and hash must
// be non-nil — both are required for AcceptOrQueue to function.
func New(s *store.Store, own peer.ID, orphanCap uint64, verify Verifier, hash Hasher) *Inbox {
	if orphanCap == 0 {
		orphanCap = DefaultOrphanCap
	}
	perPeer := orphanCap
	if perPeer > DefaultPerPeerOrphanCap {
		perPeer = DefaultPerPeerOrphanCap
	}
	return &Inbox{
		store:            s,
		ownPeerID:        own,
		perPeerOrphanCap: perPeer,
		orphanCap:        orphanCap,
		verify:           verify,
		hash:             hash,
	}
}

// AcceptOrQueue is the main entry point. See the package doc for the
// full flow. Returns nil on success regardless of accept-vs-queue
// outcome; returns a specific error on rejection or programming
// failure so tests can pin behaviour.
func (i *Inbox) AcceptOrQueue(ctx context.Context, entry *pb.SignedEntry) error {
	if entry == nil {
		return fmt.Errorf("%w: nil entry", ErrInvalidEntry)
	}

	raterPeerIDBytes, err := raterPeerID(entry)
	if err != nil {
		return err
	}

	// Self-drop BEFORE signature work — cheap and a hot path for nodes
	// that publish and subscribe on the same topic.
	if peer.ID(raterPeerIDBytes) == i.ownPeerID {
		return ErrSelfMessage
	}

	ok, err := i.verify(entry)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}
	if !ok {
		return ErrSignatureInvalid
	}

	// Fix-3: enforce the proto-level "receivers MUST reject" rules
	// AFTER signature verification (so the cost of validation only
	// applies to authenticated content).
	if err := validatePayload(entry); err != nil {
		return err
	}

	hash := i.hash(entry)
	var zero [32]byte
	if hash == zero {
		return fmt.Errorf("%w: empty hash from EntryHash", ErrInvalidEntry)
	}

	// Dedup against both already-accepted entries and orphans.
	already, err := i.store.HasInboxEntry(ctx, raterPeerIDBytes, hash)
	if err != nil {
		return fmt.Errorf("dedup check: %w", err)
	}
	if already {
		return nil
	}
	isOrphan, err := i.store.HasInboxOrphan(ctx, raterPeerIDBytes, hash)
	if err != nil {
		return fmt.Errorf("orphan dedup check: %w", err)
	}
	if isOrphan {
		return nil
	}

	prevHash := prevHashOf(entry)

	chainHead, hasChain, err := i.store.GetInboxChainHead(ctx, raterPeerIDBytes)
	if err != nil {
		return fmt.Errorf("chain head: %w", err)
	}

	// Re-marshal the entire SignedEntry (with signature included) for
	// on-disk storage; this is what we'll forward to other peers when
	// they fetch via /agentfm/ledger-fetch/1.0.0 in P2-5.
	payload, err := proto.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	// Chain-extension decision.
	switch {
	case !hasChain && prevHash == zero:
		// First entry from a never-seen peer; prev_hash MUST be all-zero
		// per Rating/Comment doc comment. Accept.
		return i.acceptAndPromote(ctx, raterPeerIDBytes, hash, prevHash, payload)
	case hasChain && prevHash == chainHead:
		// Standard extension of the rater's known chain. The store does an
		// atomic compare-and-extend, so if a concurrent accept advanced the
		// head between our read above and the insert, we lost the race:
		// queue as an orphan rather than accept a fork.
		if err := i.acceptAndPromote(ctx, raterPeerIDBytes, hash, prevHash, payload); err != nil {
			if errors.Is(err, store.ErrInboxChainMismatch) {
				return i.queueOrphan(ctx, raterPeerIDBytes, hash, prevHash, payload)
			}
			return err
		}
		return nil
	default:
		// Either we have a chain head and this entry doesn't extend it,
		// or we haven't seen this peer and they claim a non-zero
		// prev_hash. In both cases, queue as orphan and hope the parent
		// arrives later.
		return i.queueOrphan(ctx, raterPeerIDBytes, hash, prevHash, payload)
	}
}

// acceptAndPromote inserts the entry into inbox_entries and then
// walks the orphan queue depth-first, promoting any orphan that names
// this entry's hash as its parent. Each promoted orphan can in turn
// unblock its own grandchildren, so we loop until no more promotions
// happen on a single iteration.
func (i *Inbox) acceptAndPromote(
	ctx context.Context,
	peerIDBytes []byte,
	hash, prevHash [32]byte,
	payload []byte,
) error {
	if err := i.store.InsertInboxEntry(ctx, peerIDBytes, hash, prevHash, payload); err != nil {
		return fmt.Errorf("insert inbox entry: %w", err)
	}

	// BFS over orphans whose prev_hash matches the just-accepted hash,
	// then their children, then their children's children, ...
	queue := [][32]byte{hash}
	for len(queue) > 0 {
		parentHash := queue[0]
		queue = queue[1:]

		orphans, err := i.store.FindInboxOrphansAwaiting(ctx, peerIDBytes, parentHash)
		if err != nil {
			return fmt.Errorf("find orphans: %w", err)
		}
		for _, o := range orphans {
			err := i.store.InsertInboxEntry(ctx, peerIDBytes, o.Hash, o.PrevHash, o.Payload)
			if errors.Is(err, store.ErrInboxChainMismatch) {
				// A sibling already extended this parent; o is a fork at
				// this chain position and can never promote. Drop it rather
				// than absorbing the fork into inbox_entries.
				slog.Warn("inbox: dropping forked orphan at chain position (equivocation)",
					slog.String("peer", peer.ID(peerIDBytes).String()),
				)
				if derr := i.store.DeleteInboxOrphan(ctx, peerIDBytes, o.Hash); derr != nil {
					slog.Warn("inbox: failed to drop forked orphan",
						slog.Any(obs.FieldErr, derr))
				}
				continue
			}
			if err != nil {
				slog.Warn("inbox: failed to promote orphan",
					slog.Any(obs.FieldErr, err),
					slog.String("peer", peer.ID(peerIDBytes).String()),
				)
				continue
			}
			if err := i.store.DeleteInboxOrphan(ctx, peerIDBytes, o.Hash); err != nil {
				slog.Warn("inbox: failed to remove promoted orphan",
					slog.Any(obs.FieldErr, err))
				continue
			}
			queue = append(queue, o.Hash)
		}
	}
	return nil
}

// queueOrphan adds an out-of-order entry to inbox_orphans, subject to
// the global cap.
func (i *Inbox) queueOrphan(
	ctx context.Context,
	peerIDBytes []byte,
	hash, prevHash [32]byte,
	payload []byte,
) error {
	// Per-peer quota first: one sybil rater must not be able to consume the
	// whole global cap and starve every other peer's out-of-order delivery.
	perPeer, err := i.store.CountInboxOrphansForPeer(ctx, peerIDBytes)
	if err != nil {
		return fmt.Errorf("per-peer orphan count: %w", err)
	}
	if perPeer >= i.perPeerOrphanCap {
		return ErrOrphanCapExceeded
	}
	count, err := i.store.CountInboxOrphans(ctx)
	if err != nil {
		return fmt.Errorf("orphan count: %w", err)
	}
	if count >= i.orphanCap {
		return ErrOrphanCapExceeded
	}
	if err := i.store.InsertInboxOrphan(ctx, peerIDBytes, hash, prevHash, payload); err != nil {
		return fmt.Errorf("insert orphan: %w", err)
	}
	return nil
}

// HasEntry returns true if (raterID, entryHash) is already accepted
// (not orphaned). Exposed for tests and downstream API surface.
func (i *Inbox) HasEntry(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error) {
	return i.store.HasInboxEntry(ctx, raterID, entryHash)
}

// IsOrphan returns true if (raterID, entryHash) is currently waiting
// in the orphan queue. Exposed for tests.
func (i *Inbox) IsOrphan(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error) {
	return i.store.HasInboxOrphan(ctx, raterID, entryHash)
}

// raterPeerID extracts the rater_peer_id bytes from either oneof body.
func raterPeerID(entry *pb.SignedEntry) ([]byte, error) {
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating == nil {
			return nil, fmt.Errorf("%w: empty Rating", ErrInvalidEntry)
		}
		return body.Rating.RaterPeerId, nil
	case *pb.SignedEntry_Comment:
		if body.Comment == nil {
			return nil, fmt.Errorf("%w: empty Comment", ErrInvalidEntry)
		}
		return body.Comment.RaterPeerId, nil
	default:
		return nil, fmt.Errorf("%w: oneof body unset", ErrInvalidEntry)
	}
}

// prevHashOf reads the prev_hash field from either oneof body and
// converts it to a fixed [32]byte. On malformed input (wrong length)
// returns zero, which the caller treats as "looks like first entry."
func prevHashOf(entry *pb.SignedEntry) [32]byte {
	var out [32]byte
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating != nil && len(body.Rating.PrevHash) == 32 {
			copy(out[:], body.Rating.PrevHash)
		}
	case *pb.SignedEntry_Comment:
		if body.Comment != nil && len(body.Comment.PrevHash) == 32 {
			copy(out[:], body.Comment.PrevHash)
		}
	}
	return out
}

// validatePayload enforces the proto-level invariants the wire
// contract demands receivers reject (Fix-3 audit finding):
//
//   - prev_hash MUST be exactly 32 bytes (SHA-256)
//   - timestamp_unix_ns MUST be > 0
//   - Rating.score MUST be in [-1.0, +1.0] AND not NaN/Inf
//   - Rating.dimension MUST be non-empty
//   - Comment.text_cid MUST be non-empty (the body is content-addressed)
//
// Returns ErrInvalidPayload-wrapped error on the first violation.
func validatePayload(entry *pb.SignedEntry) error {
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		r := body.Rating
		if r == nil {
			return fmt.Errorf("%w: nil Rating", ErrInvalidPayload)
		}
		if len(r.PrevHash) != 32 {
			return fmt.Errorf("%w: prev_hash len=%d, want 32", ErrInvalidPayload, len(r.PrevHash))
		}
		if r.TimestampUnixNs <= 0 {
			return fmt.Errorf("%w: timestamp_unix_ns=%d", ErrInvalidPayload, r.TimestampUnixNs)
		}
		// Strict bounds + NaN/Inf rejection. Use math.IsNaN/IsInf
		// rather than direct comparison so the score field can't
		// poison downstream aggregation.
		if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
			return fmt.Errorf("%w: score=NaN/Inf", ErrInvalidPayload)
		}
		if r.Score < -1.0 || r.Score > 1.0 {
			return fmt.Errorf("%w: score=%v out of [-1,+1]", ErrInvalidPayload, r.Score)
		}
		if r.Dimension == "" {
			return fmt.Errorf("%w: empty dimension", ErrInvalidPayload)
		}
		return nil
	case *pb.SignedEntry_Comment:
		c := body.Comment
		if c == nil {
			return fmt.Errorf("%w: nil Comment", ErrInvalidPayload)
		}
		if len(c.PrevHash) != 32 {
			return fmt.Errorf("%w: prev_hash len=%d, want 32", ErrInvalidPayload, len(c.PrevHash))
		}
		if c.TimestampUnixNs <= 0 {
			return fmt.Errorf("%w: timestamp_unix_ns=%d", ErrInvalidPayload, c.TimestampUnixNs)
		}
		if len(c.TextCid) == 0 {
			return fmt.Errorf("%w: empty text_cid", ErrInvalidPayload)
		}
		return nil
	default:
		return fmt.Errorf("%w: oneof body unset", ErrInvalidEntry)
	}
}

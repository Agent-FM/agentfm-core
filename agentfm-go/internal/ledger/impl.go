package ledger

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agentfm/internal/ledger/merkle"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/obs"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// ledgerImpl is the production Ledger implementation. One per process
// (created at boot by boss/worker bootstrap). Goroutine-safe under
// the single-writer pattern enforced by store.Store: Append + WriteHead
// serialise on the store's mutex, reads scale freely.
type ledgerImpl struct {
	store  *store.Store
	key    crypto.PrivKey
	peerID peer.ID

	// mu guards tree, lastHead, and topic-close ordering.
	// Reads of the in-memory tree happen during Append (to compute
	// prev_hash + new root) and during Head (to surface the current
	// state) — both go through Append-serialised paths plus this lock.
	mu       sync.Mutex
	tree     *merkle.Tree
	lastHead *pb.LogHead // nil until first Append in this process

	// pubsub fields. topic is nil when ps was passed nil to New —
	// local-only mode for tests.
	ps    *pubsub.PubSub
	topic *pubsub.Topic
}

// newImpl is the real constructor behind the package-level New.
// Separated so tests in the same package can poke at the concrete type
// if needed; today they go through the interface.
func newImpl(path string, key crypto.PrivKey, ps *pubsub.PubSub) (Ledger, error) {
	if key == nil {
		return nil, errors.New("ledger: nil key")
	}
	pid, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("ledger: derive peer id from key: %w", err)
	}

	s, err := store.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: open store: %w", err)
	}

	tree, err := rebuildTreeFromStore(context.Background(), s)
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("ledger: rebuild merkle tree: %w", err)
	}

	l := &ledgerImpl{
		store:  s,
		key:    key,
		peerID: pid,
		tree:   tree,
		ps:     ps,
	}

	if ps != nil {
		topic, err := ps.Join(network.FeedbackTopic)
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("ledger: join feedback topic: %w", err)
		}
		l.topic = topic
	}

	// Best-effort: seed lastHead from the on-disk head so Head() returns
	// a valid snapshot immediately after a restart, before any Append.
	if err := l.loadLastHead(context.Background()); err != nil {
		slog.Warn("ledger: could not load persisted head; will produce one on next Append",
			slog.Any(obs.FieldErr, err))
	}

	return l, nil
}

// rebuildTreeFromStore walks every persisted entry in idx order and
// re-Appends its hash to a fresh Merkle tree. The store guarantees
// idx-monotonic order; the tree guarantees the same Root we had before
// shutdown, because Merkle hashing is deterministic.
func rebuildTreeFromStore(ctx context.Context, s *store.Store) (*merkle.Tree, error) {
	t := merkle.New()
	err := s.IterateEntries(ctx, 1, func(e *store.Entry) error {
		t.Append(e.Hash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// loadLastHead reads the most recent signed LogHead from the store
// into l.lastHead. Returns nil and leaves lastHead nil if there is no
// head on disk (fresh ledger).
func (l *ledgerImpl) loadLastHead(ctx context.Context) error {
	row, err := l.store.LatestHead(ctx)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	var head pb.LogHead
	if err := proto.Unmarshal(row.HeadBlob, &head); err != nil {
		return fmt.Errorf("unmarshal head_blob: %w", err)
	}
	l.mu.Lock()
	l.lastHead = &head
	l.mu.Unlock()
	return nil
}

// Append signs payload, persists it, updates the Merkle tree, signs +
// persists a new LogHead, and publishes the entry on the feedback
// topic. Gossip publish is best-effort: a publish failure is logged
// but does NOT roll back the local persist — losing a user-submitted
// rating because of a transient network blip is unacceptable.
func (l *ledgerImpl) Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	if payload == nil {
		return [32]byte{}, errors.New("ledger: nil payload")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	prev := l.tree.LastLeafHash()

	if err := SignEntry(l.key, payload, prev); err != nil {
		return [32]byte{}, fmt.Errorf("sign entry: %w", err)
	}

	h := EntryHash(payload)
	var zero [32]byte
	if h == zero {
		// EntryHash returning zero means CanonicalSignedEntry failed —
		// almost certainly a programming bug (e.g. an empty body slipped
		// through SignEntry's body check). Fail loudly.
		return [32]byte{}, errors.New("ledger: EntryHash returned zero; malformed entry")
	}

	kind, sig, err := extractKindAndSig(payload)
	if err != nil {
		return [32]byte{}, err
	}
	canonicalPayload, err := pb.CanonicalSignedEntry(payload)
	if err != nil {
		return [32]byte{}, fmt.Errorf("canonical payload: %w", err)
	}
	// Persist the SignedEntry envelope WITH signature included — the
	// canonical bytes strip it, but the on-disk row needs the full
	// signed object so other peers can fetch & verify.
	storedPayload, err := proto.Marshal(payload)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshal payload for store: %w", err)
	}
	_ = canonicalPayload // used by signing path above; kept here as a comment anchor.

	if _, err := l.store.AppendEntry(ctx, h, prev, kind, storedPayload, sig); err != nil {
		return [32]byte{}, fmt.Errorf("persist entry: %w", err)
	}

	// Now that the entry is durable, update the in-memory tree and
	// build a new head. A crash between AppendEntry and WriteHead is
	// recoverable: rebuildTreeFromStore re-derives the same root on
	// next open, and loadLastHead simply finds an older head. The next
	// Append will produce a fresh head consistent with the recovered
	// tree.
	l.tree.Append(h)
	head, err := l.signNewHead()
	if err != nil {
		return [32]byte{}, fmt.Errorf("sign new head: %w", err)
	}
	headBlob, err := proto.Marshal(head)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshal head: %w", err)
	}
	if err := l.store.WriteHead(ctx, head.TreeSize, root32(head.RootHash), head.TimestampUnixNs, headBlob); err != nil {
		return [32]byte{}, fmt.Errorf("persist head: %w", err)
	}
	l.lastHead = head

	// Publish — best effort. Marshal a fresh copy (no internal pointers
	// that callers might mutate after this returns).
	if l.topic != nil {
		if err := l.topic.Publish(ctx, storedPayload); err != nil {
			slog.Warn("ledger: gossip publish failed (entry already persisted)",
				slog.Any(obs.FieldErr, err),
				slog.String("topic", network.FeedbackTopic),
				slog.Uint64("tree_size", head.TreeSize))
		}
	}

	return h, nil
}

// signNewHead builds and signs a LogHead snapshotting the current tree.
// WitnessSigs and RekorAnchor are left empty — they're filled in by
// later phases (P2-*, P5-3). Caller must hold l.mu.
func (l *ledgerImpl) signNewHead() (*pb.LogHead, error) {
	root := l.tree.Root()
	head := &pb.LogHead{
		PeerId:          []byte(l.peerID),
		TreeSize:        l.tree.Size(),
		RootHash:        root[:],
		TimestampUnixNs: time.Now().UnixNano(),
	}
	canonical, err := pb.CanonicalLogHead(head)
	if err != nil {
		return nil, fmt.Errorf("canonical head: %w", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := l.key.Sign(digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign head digest: %w", err)
	}
	head.Signature = sig
	return head, nil
}

// Head returns the most recent signed LogHead. Returns nil, nil if no
// entries have ever been appended in this process AND no head was on
// disk at Open.
func (l *ledgerImpl) Head(ctx context.Context) (*pb.LogHead, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastHead == nil {
		return nil, nil
	}
	// Clone before returning so callers cannot mutate the cached head.
	out, _ := proto.Clone(l.lastHead).(*pb.LogHead)
	return out, nil
}

// Prove is reserved for P2-5 (inclusion proofs over the merkle tree).
func (l *ledgerImpl) Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error) {
	return nil, ErrNotImplemented
}

// VerifyEntry is reserved for P1-5 (subscribe + verify + inbox).
func (l *ledgerImpl) VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error {
	return ErrNotImplemented
}

// Close releases the store and the topic. Idempotent: the underlying
// store handles repeated Close calls cleanly, and a nil topic is just
// skipped.
func (l *ledgerImpl) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var firstErr error
	if l.topic != nil {
		if err := l.topic.Close(); err != nil {
			firstErr = fmt.Errorf("close topic: %w", err)
		}
		l.topic = nil
	}
	if err := l.store.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close store: %w", err)
	}
	return firstErr
}

// extractKindAndSig pulls the store.Kind tag and the raw signature out
// of a SignedEntry, regardless of whether it carries a Rating or a
// Comment. Both inner types expose Signature; this routes to the right
// one.
func extractKindAndSig(entry *pb.SignedEntry) (store.Kind, []byte, error) {
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating == nil {
			return 0, nil, ErrUnsetBody
		}
		return store.KindRating, body.Rating.Signature, nil
	case *pb.SignedEntry_Comment:
		if body.Comment == nil {
			return 0, nil, ErrUnsetBody
		}
		return store.KindComment, body.Comment.Signature, nil
	default:
		return 0, nil, ErrUnsetBody
	}
}

// root32 coerces a proto bytes field back into a fixed [32]byte. The
// canonical head's RootHash is always 32 bytes (see merkle.HashChildren
// and CHECK constraint in migrations/001_init.sql), but the proto field
// type is []byte so we need a small conversion.
func root32(b []byte) [32]byte {
	var out [32]byte
	copy(out[:], b)
	return out
}

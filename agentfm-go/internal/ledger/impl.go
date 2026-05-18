package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agentfm/internal/ledger/inbox"
	"agentfm/internal/ledger/merkle"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/obs"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
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
	sub   *pubsub.Subscription

	// Equivocation topic (P2-3): publishing handle + subscription. The
	// witness role publishes alerts here when it catches an
	// equivocation; every ledger subscribes so the equivocator marker
	// propagates mesh-wide.
	equivTopic *pubsub.Topic
	equivSub   *pubsub.Subscription
	equivDone  chan struct{}

	// inbox owns accept/orphan/promote for entries gossiped by OTHER
	// peers. Always non-nil after newImpl returns (the store-backed
	// implementation has no init that can fail at this point).
	inbox *inbox.Inbox

	// Subscriber goroutine lifecycle. Only populated when ps != nil.
	subCtx    context.Context
	subCancel context.CancelFunc
	subDone   chan struct{}

	// fetchHost tracks the host LedgerFetchProtocol was registered
	// on (P2-5) so Close can unregister cleanly.
	fetchHost host.Host
}

// newImpl is the real constructor behind the package-level New /
// NewWithOptions. Separated so tests in the same package can poke at
// the concrete type if needed; today they go through the interface.
func newImpl(path string, key crypto.PrivKey, ps *pubsub.PubSub, opts Options) (Ledger, error) {
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
		inbox:  inbox.New(s, pid, 0, VerifyEntry, EntryHash), // 0 → DefaultOrphanCap
	}
	if opts.Host != nil {
		// P2-5: serve LedgerFetchProtocol so other peers can pull
		// our entries for inclusion-proof walks. Registered on the
		// host directly; no separate lifecycle goroutine needed —
		// libp2p invokes handleFetch on a new goroutine per stream.
		l.startFetchHandler(opts.Host)
		// P5-1: serve HeadFetchProtocol so a restarting boss can
		// bound its catch-up window against the relay's signed head.
		l.startHeadFetchHandler(opts.Host)
		l.fetchHost = opts.Host
	}

	if ps != nil {
		topic, err := ps.Join(network.FeedbackTopic)
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("ledger: join feedback topic: %w", err)
		}
		l.topic = topic

		sub, err := topic.Subscribe()
		if err != nil {
			_ = topic.Close()
			_ = s.Close()
			return nil, fmt.Errorf("ledger: subscribe feedback topic: %w", err)
		}
		l.sub = sub
		l.subCtx, l.subCancel = context.WithCancel(context.Background())
		l.subDone = make(chan struct{})
		// Pass sub + subCtx + subDone as args so the goroutine holds
		// stable references — Close() races to nil the struct fields
		// during shutdown, and we don't want the goroutine reading
		// through the struct concurrently.
		go l.runSubscriber(l.subCtx, sub, l.subDone)

		// P2-3: also join the EquivocationTopic so this ledger reacts
		// to alerts published by witnesses anywhere in the mesh.
		equivTopic, err := ps.Join(network.EquivocationTopic)
		if err != nil {
			// Best-effort: a failure to join the equivocation topic
			// must not prevent the ledger from coming up. Log it and
			// continue without alert reception.
			slog.Warn("ledger: join equivocation topic failed",
				slog.Any(obs.FieldErr, err),
				slog.String("topic", network.EquivocationTopic))
		} else {
			l.equivTopic = equivTopic
			equivSub, err := equivTopic.Subscribe()
			if err != nil {
				slog.Warn("ledger: subscribe equivocation topic failed",
					slog.Any(obs.FieldErr, err))
				_ = equivTopic.Close()
				l.equivTopic = nil
			} else {
				l.equivSub = equivSub
				l.equivDone = make(chan struct{})
				go l.runEquivSubscriber(l.subCtx, equivSub, l.equivDone)
			}
		}
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

	// Phase 1 (under lock): sign + persist the entry, advance the
	// in-memory tree, sign a new head. Keeping all the local-state
	// mutations in one critical section preserves the "single writer"
	// invariant the store relies on.
	l.mu.Lock()
	prev := l.tree.LastLeafHash()
	if err := SignEntry(l.key, payload, prev); err != nil {
		l.mu.Unlock()
		return [32]byte{}, fmt.Errorf("sign entry: %w", err)
	}
	h := EntryHash(payload)
	var zero [32]byte
	if h == zero {
		l.mu.Unlock()
		return [32]byte{}, errors.New("ledger: EntryHash returned zero; malformed entry")
	}
	kind, sig, err := extractKindAndSig(payload)
	if err != nil {
		l.mu.Unlock()
		return [32]byte{}, err
	}
	// Persist the SignedEntry envelope WITH signature included — the
	// canonical bytes strip it, but the on-disk row needs the full
	// signed object so other peers can fetch & verify.
	storedPayload, err := proto.Marshal(payload)
	if err != nil {
		l.mu.Unlock()
		return [32]byte{}, fmt.Errorf("marshal payload for store: %w", err)
	}
	if _, err := l.store.AppendEntry(ctx, h, prev, kind, storedPayload, sig); err != nil {
		l.mu.Unlock()
		return [32]byte{}, fmt.Errorf("persist entry: %w", err)
	}
	l.tree.Append(h)
	head, err := l.signNewHead()
	if err != nil {
		l.mu.Unlock()
		return [32]byte{}, fmt.Errorf("sign new head: %w", err)
	}
	l.mu.Unlock()

	// Phase 2 (under lock): persist the head and publish. We re-acquire
	// because lastHead is read by Head() and we want a clean
	// (old head → new head) transition.
	headBlob, err := proto.Marshal(head)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshal head: %w", err)
	}
	l.mu.Lock()
	if err := l.store.WriteHead(ctx, head.TreeSize, root32(head.RootHash), head.TimestampUnixNs, headBlob); err != nil {
		l.mu.Unlock()
		return [32]byte{}, fmt.Errorf("persist head: %w", err)
	}
	l.lastHead = head
	l.mu.Unlock()

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

// Prove builds an RFC 6962 inclusion proof for entryHash against the
// current Head. The entry must be in this ledger's OWN log
// (`entries` table). Returns ErrEntryNotInLog if the hash is unknown.
//
// The returned InclusionProof carries:
//   - the original SignedEntry envelope (unmarshalled from the stored payload)
//   - the entry's 0-based position
//   - the audit_path (RFC 6962 sibling hashes leaf→root)
//   - the LogHead the proof is anchored to (current Head)
//
// Callers MUST treat the head's (root, size) as authoritative —
// inclusion proofs are size-bound, so verifying with a different size
// is undefined behaviour (see VerifyInclusion's SECURITY BOUNDARY).
func (l *ledgerImpl) Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Fix-5: O(log n) lookup via the entries_hash_idx index. Replaces
	// the prior full-scan IterateEntries — important once Prove is on
	// the dispatch hot path.
	found, err := l.store.GetEntryByHash(ctx, entryHash)
	if err != nil {
		if errors.Is(err, store.ErrEntryNotFound) {
			return nil, fmt.Errorf("%w: hash=%x", ErrEntryNotInLog, entryHash)
		}
		return nil, fmt.Errorf("lookup entry: %w", err)
	}

	// idx in the store is 1-based; Merkle position is 0-based.
	pos := found.Idx - 1
	audit, err := l.tree.InclusionProof(pos)
	if err != nil {
		return nil, fmt.Errorf("merkle proof: %w", err)
	}
	if l.lastHead == nil {
		return nil, errors.New("ledger: no signed head available; Prove requires at least one Append")
	}

	// Unmarshal the stored payload into the SignedEntry wrapper.
	signed := &pb.SignedEntry{}
	if err := proto.Unmarshal(found.Payload, signed); err != nil {
		return nil, fmt.Errorf("unmarshal stored entry: %w", err)
	}

	proof := &pb.InclusionProof{
		Position:  pos,
		AuditPath: make([][]byte, len(audit)),
		LogHead:   protoCloneLogHead(l.lastHead),
	}
	for i, h := range audit {
		bs := make([]byte, 32)
		copy(bs, h[:])
		proof.AuditPath[i] = bs
	}
	if signed.GetBody() == nil {
		return nil, ErrUnsetBody
	}
	proof.Entry = signed
	return proof, nil
}

// errStopIterate is a sentinel passed back from IterateEntries
// callbacks to stop the scan early without signalling a real error.
var errStopIterate = errors.New("stop iterate")

// ErrEntryNotInLog is returned by Prove when the requested entry hash
// is not present in this ledger's own append-only log.
var ErrEntryNotInLog = errors.New("ledger: entry not in local log")

// protoCloneLogHead is a tiny helper because proto.Clone returns
// proto.Message and a type-asserted return point is awkward to inline.
func protoCloneLogHead(h *pb.LogHead) *pb.LogHead {
	c, _ := proto.Clone(h).(*pb.LogHead)
	return c
}

// InboxHas reports whether the inbox already holds (raterID, entryHash)
// in its accepted set. Returns false if the inbox is not yet wired
// (defensive — shouldn't happen post-newImpl).
func (l *ledgerImpl) InboxHas(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error) {
	if l.inbox == nil {
		return false, nil
	}
	return l.inbox.HasEntry(ctx, raterID, entryHash)
}

// IsEquivocator delegates to the store-level equivocators query.
func (l *ledgerImpl) IsEquivocator(ctx context.Context, peerID []byte) (bool, error) {
	return l.store.IsEquivocator(ctx, peerID)
}

// Store returns the underlying SQLite store. Use for test helpers and
// read-only reputation engine walks only — do not write via this handle.
func (l *ledgerImpl) Store() *store.Store {
	return l.store
}

// VerifyEntry routes an entry through the inbox accept/orphan/promote
// path. knownHead is reserved for stricter inclusion-proof validation
// in P2-5; ignored in P1-5.
//
// Returns nil whether the entry was accepted, deduped, or queued as an
// orphan. Returns a typed inbox.Err* for verification failures or
// programming errors so callers can pin specific behaviour in tests.
func (l *ledgerImpl) VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error {
	_ = knownHead
	if l.inbox == nil {
		return ErrNotImplemented
	}
	return l.inbox.AcceptOrQueue(ctx, entry)
}

// acceptLocalAlert verifies an EquivocationAlert end-to-end and, on
// success, marks the offender as a permanent equivocator. The full
// validation chain (Fix-2 audit finding):
//
//   1. WitnessSignature is valid for WitnessPeerID.
//   2. HeadA and HeadB are non-nil and structurally sane.
//   3. Both heads carry valid peer-own signatures from alert.PeerId
//      (so a rogue witness cannot forge head pairs).
//   4. The heads ACTUALLY conflict — either same TreeSize with
//      different RootHash, OR overlapping size where the proof of
//      extension would fail. Same exact head twice is NOT an
//      equivocation (a witness sometimes self-emits this on
//      bad-head-sig cases; we accept the rest of the validation but
//      treat it as "no offence" rather than poisoning the marker).
//
// Returns nil on accept-and-mark (or on idempotent no-op when the
// alert is internally consistent but headA == headB). Returns a
// specific error when validation fails; the marker is NOT written.
func (l *ledgerImpl) acceptLocalAlert(ctx context.Context, alert *pb.EquivocationAlert) error {
	if alert == nil {
		return errors.New("nil alert")
	}
	if alert.HeadA == nil || alert.HeadB == nil {
		return errors.New("alert missing HeadA/HeadB")
	}

	// (1) Witness signature.
	witnessID, err := peer.IDFromBytes(alert.WitnessPeerId)
	if err != nil {
		return fmt.Errorf("invalid witness id: %w", err)
	}
	witnessPub, err := witnessID.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extract witness pubkey: %w", err)
	}
	canonical, err := pb.CanonicalEquivocationAlert(alert)
	if err != nil {
		return fmt.Errorf("canonical alert: %w", err)
	}
	digest := sha256.Sum256(canonical)
	ok, err := witnessPub.Verify(digest[:], alert.WitnessSignature)
	if err != nil {
		return fmt.Errorf("verify witness sig: %w", err)
	}
	if !ok {
		return errors.New("alert witness signature invalid")
	}

	// (3) Both heads must be signed by alert.PeerId. Otherwise a
	// rogue witness could forge two unrelated heads and brand any
	// peer as an equivocator.
	offenderID, err := peer.IDFromBytes(alert.PeerId)
	if err != nil {
		return fmt.Errorf("invalid offender id: %w", err)
	}
	offenderPub, err := offenderID.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("extract offender pubkey: %w", err)
	}
	if err := verifyHeadSig(offenderPub, alert.HeadA); err != nil {
		return fmt.Errorf("alert HeadA not signed by offender: %w", err)
	}
	if err := verifyHeadSig(offenderPub, alert.HeadB); err != nil {
		return fmt.Errorf("alert HeadB not signed by offender: %w", err)
	}
	// Both heads MUST claim the same peer ID as the alert.
	if !bytes.Equal(alert.HeadA.PeerId, alert.PeerId) || !bytes.Equal(alert.HeadB.PeerId, alert.PeerId) {
		return errors.New("alert head.peer_id does not match alert.peer_id")
	}

	// (4) The heads must actually conflict. If they're byte-identical,
	// this is the "bad head sig" alert variant the witness emits when
	// the offender's own sig didn't verify — already validated above
	// (step 3 would have caught a forged head), so a same-head alert
	// here means the witness emitted a duplicate. Don't mark.
	if proto.Equal(alert.HeadA, alert.HeadB) {
		return nil
	}
	// Real conflict: either same TreeSize with different RootHash
	// (classic equivocation) OR heads at different sizes (in which
	// case the witness should have flagged a consistency-proof
	// failure — we don't re-verify the proof here, just trust the
	// witness's classification once both heads are sig-valid).
	if alert.HeadA.TreeSize == alert.HeadB.TreeSize && bytes.Equal(alert.HeadA.RootHash, alert.HeadB.RootHash) {
		return errors.New("alert heads have same (size, root); no actual conflict")
	}

	blob, err := proto.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert blob: %w", err)
	}
	return l.store.MarkEquivocator(ctx, alert.PeerId, blob)
}

// verifyHeadSig returns nil iff head.Signature is a valid Ed25519
// signature by pub over the head's canonical bytes.
func verifyHeadSig(pub crypto.PubKey, head *pb.LogHead) error {
	canonical, err := pb.CanonicalLogHead(head)
	if err != nil {
		return fmt.Errorf("canonical head: %w", err)
	}
	digest := sha256.Sum256(canonical)
	ok, err := pub.Verify(digest[:], head.Signature)
	if err != nil {
		return fmt.Errorf("verify head sig: %w", err)
	}
	if !ok {
		return errors.New("head sig invalid")
	}
	return nil
}

// runEquivSubscriber drains the EquivocationTopic subscription and
// verifies + persists each alert. Self-published alerts are skipped
// (we already called acceptLocalAlert when publishing). Posts to
// done at exit so Close can wait for clean shutdown.
func (l *ledgerImpl) runEquivSubscriber(ctx context.Context, sub *pubsub.Subscription, done chan<- struct{}) {
	defer close(done)
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == l.peerID {
			continue
		}
		var alert pb.EquivocationAlert
		if err := proto.Unmarshal(msg.Data, &alert); err != nil {
			slog.Debug("ledger: alert unmarshal failed",
				slog.Any(obs.FieldErr, err))
			continue
		}
		if err := l.acceptLocalAlert(ctx, &alert); err != nil {
			slog.Debug("ledger: alert rejected",
				slog.Any(obs.FieldErr, err))
		}
	}
}

// runSubscriber drains the GossipSub subscription and forwards each
// non-self message through the inbox. The goroutine exits when ctx
// is cancelled (Close) or sub.Next returns an error (topic closed).
// All inputs are passed by argument rather than read from l.* so
// Close() can race to clear those fields without a data race here.
func (l *ledgerImpl) runSubscriber(ctx context.Context, sub *pubsub.Subscription, done chan<- struct{}) {
	defer close(done)
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			// Either we're shutting down (ctx cancelled) or the
			// subscription closed. Either way, exit.
			return
		}
		// Self-message filter — boss/telemetry uses the same pattern.
		if msg.ReceivedFrom == l.peerID {
			continue
		}
		var entry pb.SignedEntry
		if err := proto.Unmarshal(msg.Data, &entry); err != nil {
			slog.Debug("ledger: gossip unmarshal failed",
				slog.Any(obs.FieldErr, err),
				slog.String("topic", network.FeedbackTopic))
			continue
		}
		if err := l.inbox.AcceptOrQueue(ctx, &entry); err != nil {
			// Silent at info: adversarial inputs are expected. Debug
			// is appropriate so operators can correlate when needed.
			slog.Debug("ledger: gossip entry rejected by inbox",
				slog.Any(obs.FieldErr, err))
		}
	}
}

// Close releases the store, the subscription, and the topic.
// Idempotent: the underlying store handles repeated Close calls
// cleanly; nil fields are skipped.
//
// Shutdown order matters: cancel the subscriber goroutine first so it
// stops calling into store / inbox before we close them; then drop
// the subscription handle; then the topic; then the store.
func (l *ledgerImpl) Close() error {
	// Capture and clear lifecycle handles under the lock so that a
	// second Close call sees nothing to do.
	l.mu.Lock()
	subCancel := l.subCancel
	subDone := l.subDone
	sub := l.sub
	topic := l.topic
	equivSub := l.equivSub
	equivTopic := l.equivTopic
	equivDone := l.equivDone
	l.subCancel = nil
	l.subDone = nil
	l.sub = nil
	l.topic = nil
	l.equivSub = nil
	l.equivTopic = nil
	l.equivDone = nil
	l.mu.Unlock()

	// Unregister stream handlers first so no new requests arrive
	// while we're shutting down the goroutines that service them.
	if l.fetchHost != nil {
		l.stopFetchHandler(l.fetchHost)
		l.stopHeadFetchHandler(l.fetchHost)
		l.fetchHost = nil
	}

	// Drain subscriber goroutine BEFORE closing the subscription, so
	// its in-flight Next() exits cleanly via context cancellation.
	if subCancel != nil {
		subCancel()
	}
	if sub != nil {
		sub.Cancel()
	}
	if subDone != nil {
		<-subDone
	}

	if equivSub != nil {
		equivSub.Cancel()
	}
	// Fix-7: wait for the equiv subscriber goroutine to exit BEFORE
	// closing the store. Otherwise an in-flight acceptLocalAlert can
	// race the store.Close.
	if equivDone != nil {
		<-equivDone
	}

	var firstErr error
	if topic != nil {
		if err := topic.Close(); err != nil {
			firstErr = fmt.Errorf("close topic: %w", err)
		}
	}
	if equivTopic != nil {
		if err := equivTopic.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close equiv topic: %w", err)
		}
	}
	if err := l.store.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close store: %w", err)
	}
	return firstErr
}

// AcceptEntry decodes raw proto bytes as a SignedEntry and routes it
// through the inbox accept/orphan/promote path. Returns the same typed
// errors as inbox.AcceptOrQueue; returns nil on success, dedup, or
// orphan-queue (all are safe outcomes for a catch-up consumer).
func (l *ledgerImpl) AcceptEntry(ctx context.Context, payload []byte) error {
	var entry pb.SignedEntry
	if err := proto.Unmarshal(payload, &entry); err != nil {
		return fmt.Errorf("ledger: unmarshal AcceptEntry payload: %w", err)
	}
	if l.inbox == nil {
		return ErrNotImplemented
	}
	return l.inbox.AcceptOrQueue(ctx, &entry)
}

// LastInboxIdx returns 0 so that CatchUp always starts from relay entry
// 1. The inbox's built-in deduplication makes this safe — any entry the
// boss already holds will be silently no-op'd. A future optimisation
// could track the high-water relay idx in a separate store row to skip
// already-ingested pages; not needed for Phase 5.
func (l *ledgerImpl) LastInboxIdx(_ context.Context) (uint64, error) {
	return 0, nil
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

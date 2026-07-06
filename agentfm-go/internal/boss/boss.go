package boss

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"net/http"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	"agentfm/internal/ledger/store"
	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/reputation"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// MaxInflightAsyncTasks caps how many async submissions can be in flight
// simultaneously. Without a cap a flood of /api/execute/async POSTs would
// commit one libp2p dial + Podman slot + goroutine each, with no ability
// to back-pressure (the client gets 202 immediately).
const MaxInflightAsyncTasks = 256

// reputationEngineIface is the minimal interface Boss requires from the
// reputation engine. Using an interface rather than *reputation.Engine
// allows tests to inject a lightweight mock without the full store
// dependency that Recompute carries.
type reputationEngineIface interface {
	Score(peerID string) float64
	Recompute(ctx context.Context, s *store.Store) (float64, error)
}

type Boss struct {
	node          *network.MeshNode
	activeWorkers map[string]types.WorkerProfile
	lastSeen      map[string]time.Time
	lastProfile   map[string]types.WorkerProfile

	readRecomputeMu   sync.Mutex
	lastReadRecompute time.Time
	// RWMutex because the activeWorkers/lastSeen maps are read heavily
	// (HTTP /api/workers, /api/execute, /api/execute/async, the TUI redraw
	// ticker) but written only when a telemetry pulse arrives. Pure-read
	// call sites use RLock so concurrent API hits don't serialise.
	mu sync.RWMutex
	// artifactExpect maps dispatched taskIDs to the worker peer allowed
	// to deliver agentfm_artifacts/<taskID>.zip. Guarded by artifactMu
	// (not b.mu — the artifact gate is touched on every inbound artifact
	// stream and must not contend with telemetry/API reads).
	artifactMu     sync.Mutex
	artifactExpect map[string]artifactExpectation

	// asyncSlots gates spawn of background goroutines from
	// /api/execute/async. Buffered to MaxInflightAsyncTasks; non-blocking
	// send returns 503 to the client when full.
	asyncSlots chan struct{}

	// Ledger handle (P1+ wiring). Used by:
	//  - P3-3 to write L1-mismatch ratings into the ledger
	//  - P3-3 to consult IsEquivocator on dispatch
	//  - P4-2 HTTP API to expose reputation / log / proof
	// nil-safe: dispatch helpers fall back to "no-op" when unset
	// (e.g. tests that wire a Boss without the ledger).
	ledger ledger.Ledger

	// commentSubmissionHandler is populated in P4-3 when the
	// comments package is wired. Until then, the umbrella router
	// returns 501 from this hook.
	commentSubmissionHandler http.HandlerFunc

	// reputationEngine, when non-nil, is consulted by
	// buildReputationView for live EigenTrust scores. Wired by the
	// bootstrap path; tests can set it directly via the unexported
	// field for HTTP handler testing.
	reputationEngine reputationEngineIface

	// readStore is the secondary store handle for fresh-on-read
	// reputation recomputes (see Options.ReadStore).
	readStore *store.Store

	// commentsStore is the body store for comment CIDs (P4-1).
	// Used by GET /v1/peers/{id}/comments/{cid} to hydrate comment bodies.
	// Nil when the comments subsystem is not wired (e.g. in tests that
	// don't use comments).
	commentsStore *comments.Store

	// completionRater writes hourly aggregate outcome ratings into the
	// ledger. Nil when not wired (e.g. in tests that don't exercise
	// dispatch). RecordOutcome is guarded by nil-checks in dispatch handlers.
	completionRater *CompletionRatingWriter

	// reputationFloor is the minimum honesty score required to dispatch a
	// task to a worker. Always populated by NewWithOptions (default -1.0 =
	// allow all when Options.ReputationFloor is nil). Call sites read this
	// field directly with no sentinel logic — see Options.ReputationFloor
	// for the construction-time semantics.
	reputationFloor float64

	// startedAt records when this Boss was constructed. Used by the
	// /v1/about endpoint to compute uptime_seconds.
	startedAt time.Time

	// eventBus is the in-process fan-out broker for mesh events. Used by
	// the /v1/events SSE endpoint and populated by listenTelemetry /
	// ledger callbacks. Always non-nil after NewWithOptions.
	eventBus *EventBus

	// menuPickerForTest overrides the pterm interactive-select in
	// showPeerMenu. Set via SetMenuPickerForTest; nil in production.
	menuPickerForTest func([]string) (string, error)

	// peerViewHookForTest overrides the viewPeerHistory call inside
	// executeFlow. Set via SetPeerViewHookForTest; nil in production.
	peerViewHookForTest func(ctx context.Context, peerIDStr string)
}

// Options configures a new Boss. All fields are optional; New
// preserves defaults for anything left at zero.
type Options struct {
	Ledger ledger.Ledger

	// CommentSubmissionHandler, when non-nil, replaces the default
	// 501 stub for POST /v1/peers/{id}/comments (P4-3). Production
	// wiring builds this via NewCommentSubmissionHandler(store,
	// host) and passes its HandleHTTP-bound closure here.
	CommentSubmissionHandler http.HandlerFunc

	// ReputationEngine, when non-nil, is consulted by
	// /v1/peers/{id}/reputation to source scores. Bootstrap
	// typically wires this together with a background ticker that
	// calls engine.Recompute(ctx, store) every 60s.
	ReputationEngine *reputation.Engine

	// ReadStore is a store handle the boss uses to trigger
	// fresh-on-read reputation recomputes. Bootstrap opens a
	// secondary handle on the same SQLite file (WAL mode allows
	// concurrent handles) and passes it here. Without this, the
	// engine's score table only refreshes on the 60s ticker —
	// which is too coarse for demos and feels broken when a
	// strict-mode dispatch rejection doesn't immediately reflect
	// in /v1/peers/.../reputation.
	ReadStore *store.Store

	// CommentsStore, when non-nil, is the body store for comment CIDs.
	// Used by GET /v1/peers/{id}/comments/{cid} to hydrate comment text.
	CommentsStore *comments.Store

	// CompletionRater, when non-nil, receives RecordOutcome calls from
	// dispatch handlers after each attempt resolves. Bootstrap wires this
	// and calls go opts.CompletionRater.RunTicker(ctx) to emit ratings
	// every hour.
	CompletionRater *CompletionRatingWriter

	// ReputationFloor is the minimum honesty score required for dispatch.
	// Peers scoring strictly below this floor are refused. Nil means "not
	// configured" — NewWithOptions defaults to -1.0 (allow all). A non-nil
	// pointer is used as-is, including *ReputationFloor == 0 which means
	// "refuse anyone with a negative score." Use a pointer so the
	// legitimate value 0 is distinguishable from "operator did not set it."
	ReputationFloor *float64
}

func New(node *network.MeshNode) *Boss {
	return NewWithOptions(node, Options{})
}

// NewWithOptions is the production constructor. Wires ledger access,
// reputation engine, comments store, and completion rater from opts.
// Existing call sites that don't need any optional components can
// continue using New.
func NewWithOptions(node *network.MeshNode, opts Options) *Boss {
	b := &Boss{
		node:                     node,
		activeWorkers:            make(map[string]types.WorkerProfile),
		lastSeen:                 make(map[string]time.Time),
		lastProfile:              make(map[string]types.WorkerProfile),
		asyncSlots:               make(chan struct{}, MaxInflightAsyncTasks),
		ledger:                   opts.Ledger,
		commentSubmissionHandler: opts.CommentSubmissionHandler,
		readStore:                opts.ReadStore,
		commentsStore:            opts.CommentsStore,
		completionRater:          opts.CompletionRater,
		startedAt:                time.Now(),
		eventBus:                 NewEventBus(),
		// Resolve ReputationFloor once at construction. Nil = unconfigured →
		// -1.0 (allow all). Non-nil pointer is used as-is, so an explicit
		// --reputation-floor=0 cleanly means "refuse anyone with negative score."
		reputationFloor: -1.0,
	}
	if opts.ReputationFloor != nil {
		b.reputationFloor = *opts.ReputationFloor
	}
	// Assign via explicit nil-check to avoid the classic Go interface/nil gotcha:
	// a nil *reputation.Engine stored in a reputationEngineIface is a non-nil
	// interface value, causing nil-pointer panics inside Score/Recompute.
	if opts.ReputationEngine != nil {
		b.reputationEngine = opts.ReputationEngine
	}
	return b
}

func (b *Boss) Run(ctx context.Context) {
	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.NewArtifactStreamHandler(b.authorizeArtifact))

	time.Sleep(1 * time.Second)

	// Track listenTelemetry so host.Close() below waits for it to release
	// its pubsub topic + subscription. Otherwise a TUI exit may race the
	// goroutine's defer chain.
	var bgWG sync.WaitGroup
	bgWG.Add(1)
	go func() {
		defer bgWG.Done()
		b.listenTelemetry(ctx)
	}()

	for {
		worker, ok, quit := b.selectWorkerInteractive(ctx)
		if quit || ctx.Err() != nil {
			break
		}
		if !ok {
			continue
		}
		b.executeFlow(ctx, worker)
	}

	fmt.Println("\nShutting down Boss node...")
	bgWG.Wait()
	if err := b.node.Host.Close(); err != nil {
		slog.Error("host close", slog.Any(obs.FieldErr, err))
	}
}

func (b *Boss) listenTelemetry(ctx context.Context) {
	topic, err := b.node.PubSub.Join(network.TelemetryTopic)
	if err != nil {
		// Non-fatal: surface the error and return so the caller's
		// defers still run. Boss keeps working for manually-specified
		// peers but the radar will be empty.
		slog.Error("telemetry listener disabled: pubsub join", slog.Any(obs.FieldErr, err), slog.String("topic", network.TelemetryTopic))
		return
	}
	defer func() { _ = topic.Close() }()
	sub, err := topic.Subscribe()
	if err != nil {
		slog.Error("telemetry listener disabled: pubsub subscribe", slog.Any(obs.FieldErr, err))
		return
	}
	defer sub.Cancel()

	// Periodic pruner: evicts workers whose libp2p connection has dropped.
	// Centralised here so handleGetWorkers and the TUI tick can be pure
	// reads (no side effects on a GET request).
	// Tick faster than staleTelemetryTimeout (15s) so the lastSeen-based
	// eviction is responsive — a 30s tick would let stale workers linger
	// in the radar for almost half a minute past the staleness threshold.
	pruneTicker := time.NewTicker(5 * time.Second)
	defer pruneTicker.Stop()

	msgCh := make(chan *pubsubMsg, 1)
	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- &pubsubMsg{ReceivedFrom: msg.ReceivedFrom, Data: msg.Data}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pruneTicker.C:
			b.pruneDisconnectedWorkers()
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			if msg.ReceivedFrom == b.node.Host.ID() {
				continue
			}
			var profile types.WorkerProfile
			if err := json.Unmarshal(msg.Data, &profile); err == nil && profile.CPUCores > 0 {
				b.handleTelemetryProfile(profile)
			}
		}
	}
}

// pubsubMsg is a tiny shim to bridge the pubsub.Subscription.Next API into
// a select-able channel. Only the two fields the listener actually reads
// are copied across.
type pubsubMsg struct {
	ReceivedFrom peer.ID
	Data         []byte
}

// pruneDisconnectedWorkers walks activeWorkers and evicts any peer the
// libp2p host is no longer connected to. Runs under a write lock; cheap
// because the map is bounded by mesh size (typically tens of peers).
// staleTelemetryTimeout is the upper bound on how long a worker can go
// without a telemetry pulse before the pruner evicts it. Mirrors the
// previous 15s ad-hoc value that lived inline in ui.go's draw loop.
const staleTelemetryTimeout = 15 * time.Second

func (b *Boss) pruneDisconnectedWorkers() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for peerIDStr := range b.activeWorkers {
		pID, err := peer.Decode(peerIDStr)
		if err != nil {
			b.evictWorkerLocked(peerIDStr)
			continue
		}
		if b.node.Host.Network().Connectedness(pID) != netcore.Connected {
			b.evictWorkerLocked(peerIDStr)
			continue
		}
		if seen, ok := b.lastSeen[peerIDStr]; ok && now.Sub(seen) > staleTelemetryTimeout {
			b.evictWorkerLocked(peerIDStr)
		}
	}
	metrics.WorkersOnline.Set(float64(len(b.activeWorkers)))
}

// handleTelemetryProfile installs profile into activeWorkers and publishes
// a worker_online SSE event on first sighting. Re-sightings (telemetry pulses
// while the worker is already known) update the cached profile silently — the
// event is a state-transition signal, not a heartbeat.
func (b *Boss) handleTelemetryProfile(profile types.WorkerProfile) {
	b.mu.Lock()
	_, existed := b.activeWorkers[profile.PeerID]
	b.activeWorkers[profile.PeerID] = profile
	b.lastSeen[profile.PeerID] = time.Now()
	b.lastProfile[profile.PeerID] = profile
	n := len(b.activeWorkers)
	b.mu.Unlock()
	metrics.WorkersOnline.Set(float64(n))

	if !existed {
		var honesty float64
		if b.reputationEngine != nil {
			honesty = b.reputationEngine.Score(profile.PeerID)
		}
		b.publishWorkerOnline(profile.PeerID, profile.AgentName, honesty)
	}
}

// evictWorkerLocked removes peerIDStr from activeWorkers + lastSeen and
// publishes a worker_offline event. Caller MUST hold b.mu (write lock).
// Safe to call under the lock because EventBus.Publish only enqueues on
// buffered subscriber channels; it does not invoke handlers inline.
func (b *Boss) evictWorkerLocked(peerIDStr string) {
	delete(b.activeWorkers, peerIDStr)
	delete(b.lastSeen, peerIDStr)
	b.publishWorkerOffline(peerIDStr)
}

type timeoutReader struct {
	stream  netcore.Stream
	timeout time.Duration
}

func (tr *timeoutReader) Read(p []byte) (n int, err error) {
	// Refresh the read deadline on every Read. If the stream is already
	// torn down the arm fails, and surfacing that error is more honest
	// than letting the caller see a confusing downstream read failure.
	if err := tr.stream.SetReadDeadline(time.Now().Add(tr.timeout)); err != nil {
		return 0, err
	}
	return tr.stream.Read(p)
}

// shortID returns the first n runes of s, or s when shorter. Used for
// log/UI snippets where a short identifier prefix is enough for humans
// to correlate. Defends against panics on user-supplied IDs that fall
// short of the slice length.
func shortID(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

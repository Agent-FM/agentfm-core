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
	"agentfm/internal/trustedagents"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// AttestationMode controls how the Boss handles L1 image-digest
// verification on dispatch (P3-3). Three modes:
//
//   - AttestOff:    no checking; dispatch every worker that advertises
//                   the requested model/agent.
//   - AttestWarn:   look up worker's (image_ref, digest) in the trusted
//                   registry; log mismatches but still dispatch.
//   - AttestStrict: refuse dispatch on mismatch or (with
//                   RejectUnknownImages) on absent registry entries.
//                   The default in production builds.
type AttestationMode int

const (
	AttestOff AttestationMode = iota
	AttestWarn
	AttestStrict
)

// String renders the mode for log messages.
func (m AttestationMode) String() string {
	switch m {
	case AttestOff:
		return "off"
	case AttestWarn:
		return "warn"
	case AttestStrict:
		return "strict"
	default:
		return "?"
	}
}

// ParseAttestationMode parses the --attestation-mode flag value. An
// unknown string falls back to AttestWarn with a log entry — better
// than crashing a boss on a typo.
func ParseAttestationMode(s string) AttestationMode {
	switch s {
	case "off":
		return AttestOff
	case "strict":
		return AttestStrict
	default:
		return AttestWarn
	}
}

// MaxInflightAsyncTasks caps how many async submissions can be in flight
// simultaneously. Without a cap a flood of /api/execute/async POSTs would
// commit one libp2p dial + Podman slot + goroutine each, with no ability
// to back-pressure (the client gets 202 immediately).
const MaxInflightAsyncTasks = 256

type Boss struct {
	node          *network.MeshNode
	activeWorkers map[string]types.WorkerProfile
	lastSeen      map[string]time.Time
	// RWMutex because the activeWorkers/lastSeen maps are read heavily
	// (HTTP /api/workers, /api/execute, /api/execute/async, the TUI redraw
	// ticker) but written only when a telemetry pulse arrives. Pure-read
	// call sites use RLock so concurrent API hits don't serialise.
	mu sync.RWMutex
	// asyncSlots gates spawn of background goroutines from
	// /api/execute/async. Buffered to MaxInflightAsyncTasks; non-blocking
	// send returns 503 to the client when full.
	asyncSlots chan struct{}

	// P3-3: L1 verification configuration. Resolved at boss startup
	// from --attestation-mode + --trusted-agents flags. trusted is
	// always non-nil (falls back to bundled default if no flag).
	attestation         AttestationMode
	rejectUnknownImages bool
	trusted             *trustedagents.Registry

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
	reputationEngine *reputation.Engine

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
	// task to a worker. Phase 8 wires the production value (-0.5 default);
	// until then executeFlow uses a safe default of -1.0 (allow all).
	reputationFloor float64

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
	AttestationMode     AttestationMode
	RejectUnknownImages bool
	TrustedAgents       *trustedagents.Registry
	Ledger              ledger.Ledger

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
}

func New(node *network.MeshNode) *Boss {
	return NewWithOptions(node, Options{})
}

// NewWithOptions is the production constructor that wires the L1
// verification layer + ledger access. Existing call sites that
// don't care about either continue using New.
func NewWithOptions(node *network.MeshNode, opts Options) *Boss {
	trusted := opts.TrustedAgents
	if trusted == nil {
		// Fall back to bundled default so a fresh install always has
		// a working registry. Errors here are non-fatal — the boss
		// runs with an empty registry (everything is "unknown").
		if reg, err := trustedagents.LoadDefault(); err == nil {
			trusted = reg
		} else {
			slog.Warn("boss: bundled trusted-agents manifest failed to load; running with empty registry", slog.Any(obs.FieldErr, err))
		}
	}
	return &Boss{
		node:                     node,
		activeWorkers:            make(map[string]types.WorkerProfile),
		lastSeen:                 make(map[string]time.Time),
		asyncSlots:               make(chan struct{}, MaxInflightAsyncTasks),
		attestation:              opts.AttestationMode,
		rejectUnknownImages:      opts.RejectUnknownImages,
		trusted:                  trusted,
		ledger:                   opts.Ledger,
		commentSubmissionHandler: opts.CommentSubmissionHandler,
		reputationEngine:         opts.ReputationEngine,
		readStore:                opts.ReadStore,
		commentsStore:            opts.CommentsStore,
		completionRater:          opts.CompletionRater,
	}
}

func (b *Boss) Run(ctx context.Context) {
	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

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
				b.mu.Lock()
				b.activeWorkers[profile.PeerID] = profile
				b.lastSeen[profile.PeerID] = time.Now()
				n := len(b.activeWorkers)
				b.mu.Unlock()
				metrics.WorkersOnline.Set(float64(n))
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
			delete(b.activeWorkers, peerIDStr)
			delete(b.lastSeen, peerIDStr)
			continue
		}
		if b.node.Host.Network().Connectedness(pID) != netcore.Connected {
			delete(b.activeWorkers, peerIDStr)
			delete(b.lastSeen, peerIDStr)
			continue
		}
		if seen, ok := b.lastSeen[peerIDStr]; ok && now.Sub(seen) > staleTelemetryTimeout {
			delete(b.activeWorkers, peerIDStr)
			delete(b.lastSeen, peerIDStr)
		}
	}
	metrics.WorkersOnline.Set(float64(len(b.activeWorkers)))
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

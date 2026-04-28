package boss

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

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
}

func New(node *network.MeshNode) *Boss {
	return &Boss{
		node:          node,
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		asyncSlots:    make(chan struct{}, MaxInflightAsyncTasks),
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

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
)

type Boss struct {
	node          *network.MeshNode
	activeWorkers map[string]types.WorkerProfile
	lastSeen      map[string]time.Time
	// RWMutex because the activeWorkers/lastSeen maps are read heavily
	// (HTTP /api/workers, /api/execute, /api/execute/async, the TUI redraw
	// ticker) but written only when a telemetry pulse arrives. Pure-read
	// call sites use RLock so concurrent API hits don't serialise.
	mu sync.RWMutex
}

func New(node *network.MeshNode) *Boss {
	return &Boss{
		node:          node,
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
	}
}

func (b *Boss) Run(ctx context.Context) {
	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	time.Sleep(1 * time.Second)
	go b.listenTelemetry(ctx)

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

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
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

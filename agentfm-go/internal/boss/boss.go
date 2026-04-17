package boss

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/pterm/pterm"
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
		b.executeFlow(worker)
	}

	fmt.Println("\nShutting down Boss node...")
	if err := b.node.Host.Close(); err != nil {
		pterm.Error.Printfln("Host close error: %v", err)
	}
}

func (b *Boss) listenTelemetry(ctx context.Context) {
	topic, err := b.node.PubSub.Join(network.TelemetryTopic)
	if err != nil {
		// Non-fatal: surface the error and return so the caller's
		// defers still run. Boss keeps working for manually-specified
		// peers but the radar will be empty.
		pterm.Error.Printfln("Telemetry listener disabled: failed to join %q: %v", network.TelemetryTopic, err)
		return
	}
	sub, err := topic.Subscribe()
	if err != nil {
		pterm.Error.Printfln("Telemetry listener disabled: failed to subscribe: %v", err)
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
			b.mu.Unlock()
		}
	}
}

type timeoutReader struct {
	stream  netcore.Stream
	timeout time.Duration
}

func (tr *timeoutReader) Read(p []byte) (n int, err error) {
	tr.stream.SetReadDeadline(time.Now().Add(tr.timeout))
	return tr.stream.Read(p)
}

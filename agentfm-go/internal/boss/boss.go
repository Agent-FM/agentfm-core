package boss

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/pterm/pterm"
)

const AppVersion = "1.0.0"

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
		worker, ok := b.selectWorkerInteractive()
		if !ok {
			continue
		}
		b.executeFlow(worker)
	}
}

func (b *Boss) listenTelemetry(ctx context.Context) {
	topic, err := b.node.PubSub.Join(network.TelemetryTopic)
	if err != nil {
		pterm.Fatal.Println(err)
	}
	sub, err := topic.Subscribe()
	if err != nil {
		pterm.Fatal.Println(err)
	}

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

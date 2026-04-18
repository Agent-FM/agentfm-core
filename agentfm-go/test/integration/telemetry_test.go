package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// TestTelemetry_WorkerProfileFanout_Integration is the canonical integration
// test for the GossipSub telemetry layer. It wires two real libp2p hosts,
// spins up GossipSub on each, publishes a WorkerProfile from one side, and
// asserts the other side receives it — byte-for-byte — via a subscription.
//
// This exercises:
//   - network.TelemetryTopic (the production topic name)
//   - types.WorkerProfile JSON schema stability
//   - real pubsub propagation (not mocked)
//
// If this test breaks, one of the following has regressed:
//   - A worker publishing telemetry is no longer discoverable by a boss.
//   - The WorkerProfile JSON schema has diverged.
//   - GossipSub mesh formation is broken on the local TCP transport.
func TestTelemetry_WorkerProfileFanout_Integration(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	worker, boss := hosts[0], hosts[1]

	// Bound the whole test at 10s so a hung subscription can never wedge CI.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	psWorker, err := pubsub.NewGossipSub(ctx, worker, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub worker: %v", err)
	}
	psBoss, err := pubsub.NewGossipSub(ctx, boss, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub boss: %v", err)
	}

	bossTopic, err := psBoss.Join(network.TelemetryTopic)
	if err != nil {
		t.Fatalf("boss join: %v", err)
	}
	sub, err := bossTopic.Subscribe()
	if err != nil {
		t.Fatalf("boss subscribe: %v", err)
	}
	t.Cleanup(sub.Cancel)

	workerTopic, err := psWorker.Join(network.TelemetryTopic)
	if err != nil {
		t.Fatalf("worker join: %v", err)
	}

	// GossipSub needs a moment to form its mesh after both peers subscribe.
	// 200ms is the commonly-cited lower bound for deterministic delivery
	// on a two-peer mesh; we poll for mesh convergence to avoid the arbitrary
	// sleep being a flake source.
	testutil.Eventually(t, 3*time.Second, func() bool {
		return len(workerTopic.ListPeers()) >= 1
	}, "worker to see boss in GossipSub mesh")

	want := types.WorkerProfile{
		PeerID:       worker.ID().String(),
		AgentName:    "Integration Agent",
		AgentDesc:    "Test fixture",
		Model:        "llama3.2",
		Author:       "sdet-bot",
		Status:       "AVAILABLE",
		CPUCores:     8,
		CPUUsagePct:  25.5,
		RAMFreeGB:    12.3,
		CurrentTasks: 1,
		MaxTasks:     10,
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := workerTopic.Publish(ctx, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg, err := sub.Next(ctx)
	if err != nil {
		t.Fatalf("sub.Next: %v", err)
	}

	var got types.WorkerProfile
	if err := json.Unmarshal(msg.Data, &got); err != nil {
		t.Fatalf("unmarshal received msg: %v (raw=%s)", err, string(msg.Data))
	}
	if got != want {
		t.Errorf("telemetry round-trip mismatch:\n got:  %+v\n want: %+v", got, want)
	}
	if msg.ReceivedFrom != worker.ID() {
		t.Errorf("ReceivedFrom = %s, want %s", msg.ReceivedFrom, worker.ID())
	}
}

// TestTelemetry_MultipleWorkersFanOutToBoss_Integration verifies the Boss's
// telemetry listener can correctly demultiplex pulses from many workers —
// the common operational case when a Boss connects to a busy mesh.
func TestTelemetry_MultipleWorkersFanOutToBoss_Integration(t *testing.T) {
	const nWorkers = 3
	hosts := testutil.NewConnectedMesh(t, nWorkers+1)
	boss := hosts[nWorkers]
	workers := hosts[:nWorkers]

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	psBoss, err := pubsub.NewGossipSub(ctx, boss, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub boss: %v", err)
	}
	bossTopic, err := psBoss.Join(network.TelemetryTopic)
	if err != nil {
		t.Fatalf("boss join: %v", err)
	}
	sub, err := bossTopic.Subscribe()
	if err != nil {
		t.Fatalf("boss subscribe: %v", err)
	}
	t.Cleanup(sub.Cancel)

	workerTopics := make([]*pubsub.Topic, nWorkers)
	for i, w := range workers {
		pw, err := pubsub.NewGossipSub(ctx, w, pubsub.WithFloodPublish(true))
		if err != nil {
			t.Fatalf("pubsub worker[%d]: %v", i, err)
		}
		topic, err := pw.Join(network.TelemetryTopic)
		if err != nil {
			t.Fatalf("worker[%d] join: %v", i, err)
		}
		workerTopics[i] = topic
	}

	// Wait for the boss's subscription to register on every worker's mesh.
	testutil.Eventually(t, 5*time.Second, func() bool {
		for _, topic := range workerTopics {
			if len(topic.ListPeers()) < 1 {
				return false
			}
		}
		return true
	}, "all workers to see boss in GossipSub mesh")

	// Publish one profile per worker.
	wantByPeerID := make(map[string]types.WorkerProfile, nWorkers)
	for i, w := range workers {
		p := types.WorkerProfile{
			PeerID:    w.ID().String(),
			AgentName: "agent-" + string(rune('A'+i)),
			Model:     "test-model",
			CPUCores:  4,
			Status:    "AVAILABLE",
		}
		wantByPeerID[p.PeerID] = p

		data, _ := json.Marshal(p)
		if err := workerTopics[i].Publish(ctx, data); err != nil {
			t.Fatalf("worker[%d] publish: %v", i, err)
		}
	}

	// Collect exactly nWorkers distinct messages or fail.
	gotByPeerID := make(map[string]types.WorkerProfile, nWorkers)
	for len(gotByPeerID) < nWorkers {
		msg, err := sub.Next(ctx)
		if err != nil {
			t.Fatalf("sub.Next: %v", err)
		}
		var p types.WorkerProfile
		if err := json.Unmarshal(msg.Data, &p); err != nil {
			continue
		}
		gotByPeerID[p.PeerID] = p
	}

	for id, want := range wantByPeerID {
		got, ok := gotByPeerID[id]
		if !ok {
			t.Errorf("missing profile for peer %s", id)
			continue
		}
		if got != want {
			t.Errorf("peer %s: got %+v, want %+v", id, got, want)
		}
	}
}

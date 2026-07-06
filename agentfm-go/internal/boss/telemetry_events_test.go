package boss

import (
	"sync"
	"testing"
	"time"

	"agentfm/internal/types"
)

// TestHandleTelemetryProfile_PublishesWorkerOnlineOnFirstSighting pins the
// behavior the desktop relies on: the moment a new worker shows up in
// telemetry, the boss publishes a worker_online event on the EventBus so SSE
// subscribers can invalidate cached worker lists.
//
// Regression test for the "workers go on/off and need project restart to
// refresh" desktop bug — caused by publishWorkerOnline being defined but
// never invoked from the telemetry hot path.
func TestHandleTelemetryProfile_PublishesWorkerOnlineOnFirstSighting(t *testing.T) {
	b := &Boss{
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		lastProfile:   make(map[string]types.WorkerProfile),
		eventBus:      NewEventBus(),
		mu:            sync.RWMutex{},
	}

	ch := b.eventBus.Subscribe()
	defer b.eventBus.Unsubscribe(ch)

	profile := types.WorkerProfile{
		PeerID:    "12D3KooWTest1",
		AgentName: "TestAgent",
		CPUCores:  4,
	}
	b.handleTelemetryProfile(profile)

	select {
	case ev := <-ch:
		if ev.Type != "worker_online" {
			t.Fatalf("want worker_online; got %s", ev.Type)
		}
		if ev.Payload["peer_id"] != "12D3KooWTest1" {
			t.Errorf("want peer_id 12D3KooWTest1; got %v", ev.Payload["peer_id"])
		}
		if ev.Payload["agent_name"] != "TestAgent" {
			t.Errorf("want agent_name TestAgent; got %v", ev.Payload["agent_name"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no worker_online event published")
	}

	// Re-sighting the same peer (the 2s telemetry pulse fires constantly while
	// the worker is online) must NOT republish — the event represents a state
	// transition, not a telemetry tick.
	b.handleTelemetryProfile(profile)
	select {
	case ev := <-ch:
		t.Errorf("unexpected re-publish on second sighting: %v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected: no event
	}
}

// TestEvictWorker_PublishesWorkerOffline pins the eviction-side behavior:
// when a worker is removed from activeWorkers (disconnected, stale telemetry,
// undecodable peer id), the boss publishes a worker_offline event.
func TestEvictWorker_PublishesWorkerOffline(t *testing.T) {
	b := &Boss{
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		lastProfile:   make(map[string]types.WorkerProfile),
		eventBus:      NewEventBus(),
		mu:            sync.RWMutex{},
	}

	b.activeWorkers["12D3KooWTest2"] = types.WorkerProfile{PeerID: "12D3KooWTest2"}
	b.lastSeen["12D3KooWTest2"] = time.Now()

	ch := b.eventBus.Subscribe()
	defer b.eventBus.Unsubscribe(ch)

	b.mu.Lock()
	b.evictWorkerLocked("12D3KooWTest2")
	b.mu.Unlock()

	select {
	case ev := <-ch:
		if ev.Type != "worker_offline" {
			t.Fatalf("want worker_offline; got %s", ev.Type)
		}
		if ev.Payload["peer_id"] != "12D3KooWTest2" {
			t.Errorf("want peer_id 12D3KooWTest2; got %v", ev.Payload["peer_id"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no worker_offline event published")
	}

	if _, exists := b.activeWorkers["12D3KooWTest2"]; exists {
		t.Error("worker should be removed from activeWorkers")
	}
}

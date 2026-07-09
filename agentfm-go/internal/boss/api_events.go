package boss

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event is a single mesh event that can be fanned out to SSE subscribers.
type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// EventBus is an in-process fan-out broker. Subscribers get a buffered
// channel; the broker drops events on backpressure so a slow subscriber
// cannot stall the producer.
type EventBus struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

// NewEventBus creates an initialised EventBus.
func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[chan Event]struct{})}
}

// Subscribe adds the caller to the fan-out list and returns a buffered
// channel on which events will arrive. The caller MUST call Unsubscribe
// when it is done; otherwise the map grows without bound.
func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16)
	eb.mu.Lock()
	eb.subscribers[ch] = struct{}{}
	eb.mu.Unlock()
	return ch
}

// Unsubscribe removes ch from the fan-out list and closes it.
// Safe to call multiple times for the same ch (idempotent after first call).
func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	if _, ok := eb.subscribers[ch]; ok {
		delete(eb.subscribers, ch)
		close(ch)
	}
	eb.mu.Unlock()
}

// Publish sends e to every current subscriber. If a subscriber's buffer is
// full the event is dropped for that subscriber only; other subscribers are
// unaffected.
func (eb *EventBus) Publish(e Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for ch := range eb.subscribers {
		select {
		case ch <- e:
		default:
			// drop — slow subscriber; do not block the producer
		}
	}
}

// handleEvents services GET /v1/events — an SSE stream of mesh events.
// Clients connect and receive a sequence of SSE frames; each frame
// corresponds to one Event published on the EventBus. A heartbeat comment
// is emitted every 15 s so proxies do not time out idle streams.
func (b *Boss) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if b.eventBus == nil {
		http.Error(w, "events disabled", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := b.eventBus.Subscribe()
	defer b.eventBus.Unsubscribe(ch)

	// Emit initial ping so clients know the stream is alive.
	fmt.Fprintf(w, ":ready\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, more := <-ch:
			if !more {
				return
			}
			payload, _ := json.Marshal(e.Payload)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, payload)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ":heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// listenTelemetryWithEvents is a hook called by the event-publishing path
// inside listenTelemetry when a worker_online or worker_offline event fires.
// Extracted so it can be called safely even when eventBus is nil.
func (b *Boss) publishWorkerOnline(peerID, agentName string, honestyScore float64) {
	if b.eventBus == nil {
		return
	}
	b.eventBus.Publish(Event{
		Type: "worker_online",
		Payload: map[string]any{
			"peer_id":       peerID,
			"agent_name":    agentName,
			"honesty_score": honestyScore,
		},
	})
}

// publishWorkerOffline publishes a worker_offline event when a worker is pruned.
func (b *Boss) publishWorkerOffline(peerID string) {
	if b.eventBus == nil {
		return
	}
	b.eventBus.Publish(Event{
		Type:    "worker_offline",
		Payload: map[string]any{"peer_id": peerID},
	})
}

// publishEntryAppended publishes an entry_appended event when a ledger entry
// is written. subjectPID is the raw peer.ID bytes; kind is the entry type string.
func (b *Boss) publishEntryAppended(subjectPID []byte, kind string) {
	if b.eventBus == nil {
		return
	}
	b.eventBus.Publish(Event{
		Type: "entry_appended",
		Payload: map[string]any{
			"subject_peer_id": fmt.Sprintf("%x", subjectPID),
			"kind":            kind,
		},
	})
}

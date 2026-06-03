package boss

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventsSSE_PushesWorkerOnlineEvent(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	b.eventBus = NewEventBus()

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/v1/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		b.handleEventsForTest(rec, req)
		close(done)
	}()

	// Give the goroutine time to subscribe.
	time.Sleep(50 * time.Millisecond)

	b.eventBus.Publish(Event{
		Type:    "worker_online",
		Payload: map[string]any{"peer_id": "12D3KooWTest"},
	})
	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "event: worker_online") {
		t.Fatalf("expected event: worker_online; got:\n%s", body)
	}
	if !strings.Contains(body, `"peer_id":"12D3KooWTest"`) {
		t.Fatalf("expected payload; got:\n%s", body)
	}
}

func TestEventsSSE_RejectsNonGET(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/events", nil)
	b.handleEventsForTest(rec, req)
	if rec.Code != 405 {
		t.Fatalf("want 405; got %d", rec.Code)
	}
}

func TestEventsSSE_NilBusFails503(t *testing.T) {
	b := &Boss{} // no eventBus
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/events", nil)
	b.handleEventsForTest(rec, req)
	if rec.Code != 503 {
		t.Fatalf("want 503; got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEventBus_DropOnBackpressure(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Fill the channel buffer (capacity 16) + one more.
	for i := 0; i < 17; i++ {
		bus.Publish(Event{Type: "test", Payload: map[string]any{"i": i}})
	}
	// Should have received exactly 16 (the buffer size); the 17th was dropped.
	if got := len(ch); got != 16 {
		t.Errorf("expected buffer full at 16; got %d", got)
	}
}

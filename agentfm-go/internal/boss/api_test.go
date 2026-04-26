package boss

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agentfm/internal/types"
	"agentfm/test/testutil"
)

// --- CORS middleware -------------------------------------------------------

// TestCORSMiddleware_AddsHeadersOnPassthrough verifies every response carries
// the CORS headers. Any drift from the documented API contract would break
// every browser / Next.js integration overnight.
func TestCORSMiddleware_AddsHeadersOnPassthrough(t *testing.T) {
	t.Parallel()
	called := false
	wrapped := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()
	wrapped(rec, req)

	if !called {
		t.Error("next handler was not invoked on GET")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Access-Control-Allow-Methods missing POST: %q", got)
	}
}

// TestCORSMiddleware_OptionsShortCircuits: preflight OPTIONS requests must
// return 200 OK *without* invoking the wrapped handler — otherwise browsers
// see CORS errors.
func TestCORSMiddleware_OptionsShortCircuits(t *testing.T) {
	t.Parallel()
	called := false
	wrapped := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/foo", nil)
	rec := httptest.NewRecorder()
	wrapped(rec, req)

	if called {
		t.Error("wrapped handler must not run on OPTIONS preflight")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- GET /api/workers ------------------------------------------------------

// TestHandleGetWorkers_Empty: with no telemetry ever received, the endpoint
// returns success: true with an empty agents array (not null, not missing).
// Downstream clients rely on the array always being present.
func TestHandleGetWorkers_Empty(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)

	req := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	rec := httptest.NewRecorder()
	b.handleGetWorkers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	if resp["success"] != true {
		t.Errorf("success = %v, want true", resp["success"])
	}
	agents, ok := resp["agents"].([]any)
	if !ok {
		t.Fatalf("agents: not an array: %v", resp["agents"])
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

// TestHandleGetWorkers_MethodNotAllowed confirms the hardened server rejects
// non-GET verbs on the read endpoint.
func TestHandleGetWorkers_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest(http.MethodPost, "/api/workers", nil)
	rec := httptest.NewRecorder()
	b.handleGetWorkers(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// TestHandleGetWorkers_PrunesDisconnected verifies the self-cleaning
// behaviour of the endpoint: a peer that the host is not currently
// connected to gets deleted from the activeWorkers map while serving the
// request. Guards the stale-entry cleanup logic against regression.
func TestHandleGetWorkers_PrunesDisconnected(t *testing.T) {
	b := newTestBoss(t)

	// Use the peer ID of a host we close immediately, guaranteeing it is
	// not connected to b.node.Host when the handler runs.
	throwaway := testutil.NewHost(t)
	disconnectedID := throwaway.ID().String()
	_ = throwaway.Close()

	b.activeWorkers[disconnectedID] = types.WorkerProfile{
		PeerID:   disconnectedID,
		CPUCores: 4,
	}
	b.lastSeen[disconnectedID] = time.Now()

	req := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	rec := httptest.NewRecorder()
	b.handleGetWorkers(rec, req)

	if _, exists := b.activeWorkers[disconnectedID]; exists {
		t.Error("disconnected worker was not pruned")
	}
}

// TestHandleGetWorkers_HardwareStringGPU vs CPU: the response's "hardware"
// field is derived client-side from profile.HasGPU. Test both branches.
func TestHandleGetWorkers_HardwareStringBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		profile  types.WorkerProfile
		wantFrag string
	}{
		{
			name: "cpu-only",
			profile: types.WorkerProfile{
				Model:    "llama3.2",
				CPUCores: 12,
				HasGPU:   false,
			},
			wantFrag: "CPU: 12 Cores",
		},
		{
			name: "gpu",
			profile: types.WorkerProfile{
				Model:      "flux",
				HasGPU:     true,
				GPUUsedGB:  6.4,
				GPUTotalGB: 24.0,
			},
			wantFrag: "GPU VRAM: 6.4/24.0 GB",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newTestBoss(t)

			// Use an obviously-bogus peer id string. peer.Decode will fail,
			// which means the handler skips the Connectedness prune check
			// and still renders the entry in the response.
			fakeID := "not-decodable-id-" + tc.name
			tc.profile.PeerID = fakeID
			b.activeWorkers[fakeID] = tc.profile

			req := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
			rec := httptest.NewRecorder()
			b.handleGetWorkers(rec, req)

			if !strings.Contains(rec.Body.String(), tc.wantFrag) {
				t.Errorf("response missing %q; got: %s", tc.wantFrag, rec.Body.String())
			}
		})
	}
}

// --- POST /api/execute -----------------------------------------------------

// TestHandleExecuteTask_MethodNotAllowed: only POST is supported.
func TestHandleExecuteTask_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest(http.MethodGet, "/api/execute", nil)
	rec := httptest.NewRecorder()
	b.handleExecuteTask(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// TestHandleExecuteTask_InvalidJSON: malformed body → 400. Guards the
// defensive io.LimitReader + json.Decoder boundary.
func TestHandleExecuteTask_InvalidJSON(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader("this is not json"))
	rec := httptest.NewRecorder()
	b.handleExecuteTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestHandleExecuteTask_WorkerNotFound: if the Boss hasn't seen telemetry
// from the worker id, respond 404 immediately — no P2P dial attempted.
func TestHandleExecuteTask_WorkerNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]string{
		"worker_id": "12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT",
		"prompt":    "hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleExecuteTask(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestHandleExecuteTask_InvalidWorkerIDFormat: entry exists in the map but
// the key is not a valid peer.ID. peer.Decode returns error → 400.
func TestHandleExecuteTask_InvalidWorkerIDFormat(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["not-a-peer-id"] = types.WorkerProfile{PeerID: "not-a-peer-id"}

	body, _ := json.Marshal(map[string]string{
		"worker_id": "not-a-peer-id",
		"prompt":    "hello",
		"task_id":   "t_abc12345",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleExecuteTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- POST /api/execute/async -----------------------------------------------

// TestAsyncExecuteHandler_MethodNotAllowed covers the factory-returned
// handler's verb check.
func TestAsyncExecuteHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	var wg sync.WaitGroup
	h := b.asyncExecuteHandler(context.Background(), &wg)

	req := httptest.NewRequest(http.MethodGet, "/api/execute/async", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	// WG must remain at zero — no background goroutine should be spawned.
	// Wait briefly to confirm nothing ran.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitGroup has in-flight goroutines; handler must not spawn on 405")
	}
}

// TestAsyncExecuteHandler_InvalidJSON: same defensive boundary as
// /api/execute — bad JSON returns 400 without spawning a goroutine.
func TestAsyncExecuteHandler_InvalidJSON(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	var wg sync.WaitGroup
	h := b.asyncExecuteHandler(context.Background(), &wg)

	req := httptest.NewRequest(http.MethodPost, "/api/execute/async", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestAsyncExecuteHandler_WorkerNotFound: 404 before goroutine spawn —
// guards against the bug where the WaitGroup could leak when the worker id
// check fails.
func TestAsyncExecuteHandler_WorkerNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	var wg sync.WaitGroup
	h := b.asyncExecuteHandler(context.Background(), &wg)

	body, _ := json.Marshal(map[string]string{
		"worker_id":   "12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT",
		"prompt":      "hi",
		"webhook_url": "http://localhost:99999/hook",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/execute/async", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	// Confirm no goroutine was spawned.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitGroup non-zero after 404 path")
	}
}

// TestAsyncExecuteHandler_InvalidWorkerIDFormat: peer.Decode fails → 400
// and again no goroutine spawn.
func TestAsyncExecuteHandler_InvalidWorkerIDFormat(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["bogus-id"] = types.WorkerProfile{PeerID: "bogus-id"}
	var wg sync.WaitGroup
	h := b.asyncExecuteHandler(context.Background(), &wg)

	body, _ := json.Marshal(map[string]string{
		"worker_id": "bogus-id",
		"prompt":    "hi",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/execute/async", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

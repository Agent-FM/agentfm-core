package boss

import (
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
)

// seededDispatchBoss returns a Boss with one seeded online worker whose
// peer ID is a real libp2p ID (so peer.Decode succeeds and the trust gate
// runs). The returned peer-id string is the worker_id to dispatch to.
func seededDispatchBoss(t *testing.T) (*Boss, string) {
	t.Helper()
	h := testutil.NewHost(t)
	b := NewForTest(&network.MeshNode{Host: h})
	workerID := testutil.NewHost(t).ID().String()
	b.SeedWorker(types.WorkerProfile{PeerID: workerID, AgentName: "Victim", Status: "AVAILABLE"})
	return b, workerID
}

func dispatchBody(workerID, prompt string) string {
	return `{"worker_id":"` + workerID + `","prompt":"` + prompt + `","task_id":"task_trust_gate"}`
}

func TestExecute_TrustGateBlocksEquivocator(t *testing.T) {
	b, workerID := seededDispatchBoss(t)
	b.ledger = alwaysEquivocatorLedger{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(dispatchBody(workerID, "hi")))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 403 {
		t.Fatalf("equivocator dispatch: status=%d, want 403; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "equivocator") {
		t.Errorf("body should name the refusal reason; got %q", rec.Body.String())
	}
}

func TestExecute_TrustGateBlocksBelowFloor(t *testing.T) {
	b, workerID := seededDispatchBoss(t)
	b.reputationFloor = -0.5
	b.SetReputationScoreForTest(workerID, -0.9)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(dispatchBody(workerID, "hi")))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 403 {
		t.Fatalf("below-floor dispatch: status=%d, want 403; body=%q", rec.Code, rec.Body.String())
	}
}

func TestExecuteAsync_TrustGateBlocksEquivocator(t *testing.T) {
	b, workerID := seededDispatchBoss(t)
	b.ledger = alwaysEquivocatorLedger{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute/async", strings.NewReader(dispatchBody(workerID, "hi")))
	b.ServeHTTPExecuteAsync(rec, req)

	if rec.Code != 403 {
		t.Fatalf("async equivocator dispatch: status=%d, want 403; body=%q", rec.Code, rec.Body.String())
	}
}

func TestExecute_MissingWorkerIDIs400(t *testing.T) {
	b, _ := seededDispatchBoss(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(`{}`))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 400 {
		t.Fatalf("empty body: status=%d, want 400; body=%q", rec.Code, rec.Body.String())
	}
}

func TestExecute_MissingPromptIs400(t *testing.T) {
	b, workerID := seededDispatchBoss(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute",
		strings.NewReader(`{"worker_id":"`+workerID+`"}`))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 400 {
		t.Fatalf("missing prompt: status=%d, want 400; body=%q", rec.Code, rec.Body.String())
	}
}

func TestExecuteAsync_MissingWorkerIDIs400(t *testing.T) {
	b, _ := seededDispatchBoss(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute/async", strings.NewReader(`{}`))
	b.ServeHTTPExecuteAsync(rec, req)

	if rec.Code != 400 {
		t.Fatalf("async empty body: status=%d, want 400; body=%q", rec.Code, rec.Body.String())
	}
}

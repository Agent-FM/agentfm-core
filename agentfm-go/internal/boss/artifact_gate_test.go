package boss

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/test/testutil"
)

func TestAuthorizeArtifact_UnknownTaskRefused(t *testing.T) {
	b := newTestBoss(t)
	worker := testutil.NewHost(t)

	if b.authorizeArtifact("task_unknown", worker.ID()) {
		t.Fatal("expected refusal for task that was never dispatched")
	}
}

func TestAuthorizeArtifact_WrongPeerRefusedRightPeerAccepted(t *testing.T) {
	b := newTestBoss(t)
	worker := testutil.NewHost(t)
	impostor := testutil.NewHost(t)

	b.expectArtifact("task_abc", worker.ID())

	if b.authorizeArtifact("task_abc", impostor.ID()) {
		t.Fatal("expected refusal for peer that was not dispatched the task")
	}
	if !b.authorizeArtifact("task_abc", worker.ID()) {
		t.Fatal("expected the dispatched worker to be authorized")
	}
}

func TestAuthorizeArtifact_ConsumedAfterDelivery(t *testing.T) {
	b := newTestBoss(t)
	worker := testutil.NewHost(t)

	b.expectArtifact("task_once", worker.ID())

	if !b.authorizeArtifact("task_once", worker.ID()) {
		t.Fatal("first delivery should be authorized")
	}
	if b.authorizeArtifact("task_once", worker.ID()) {
		t.Fatal("second delivery for the same task should be refused")
	}
}

func TestAuthorizeArtifact_ExpiredEntryRefused(t *testing.T) {
	b := newTestBoss(t)
	worker := testutil.NewHost(t)

	b.expectArtifact("task_old", worker.ID())
	b.artifactMu.Lock()
	e := b.artifactExpect["task_old"]
	e.expires = time.Now().Add(-time.Second)
	b.artifactExpect["task_old"] = e
	b.artifactMu.Unlock()

	if b.authorizeArtifact("task_old", worker.ID()) {
		t.Fatal("expected refusal for an expired expectation")
	}
}

func TestHandleExecuteTask_UnsafeTaskIDRejected(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]string{
		"worker_id": "12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT",
		"prompt":    "hello",
		"task_id":   "../../etc/passwd",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleExecuteTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for path-traversal task_id", rec.Code)
	}
}

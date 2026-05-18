package boss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentfm/internal/types"
)

// TestVisibilityFields_Workers asserts that GET /api/workers includes
// all the new visibility fields added in sub-task 1.2.
func TestVisibilityFields_Workers(t *testing.T) {
	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{
			"peer1": {
				PeerID:           "peer1",
				AgentName:        "test-agent",
				AgentImageRef:    "ghcr.io/agentfm/test:v1",
				AgentImageDigest: "sha256:abc123",
				AgentCapability:  "code-helper",
			},
		},
		lastSeen: make(map[string]time.Time),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	b.handleGetWorkers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	agents, ok := resp["agents"].([]interface{})
	if !ok || len(agents) == 0 {
		t.Fatalf("agents field missing or empty: %v", resp)
	}

	worker, ok := agents[0].(map[string]interface{})
	if !ok {
		t.Fatalf("agent is not a map: %T", agents[0])
	}

	mustHave := []string{
		"agent_image_ref",
		"agent_image_digest",
		"agent_capability",
		"honesty_score",
		"is_equivocator",
		"dispatch_allowed",
		"online",
	}
	for _, field := range mustHave {
		if _, ok := worker[field]; !ok {
			t.Errorf("field %q missing from /api/workers response; got keys: %v", field, mapKeys(worker))
		}
	}

	if ref := worker["agent_image_ref"]; ref != "ghcr.io/agentfm/test:v1" {
		t.Errorf("agent_image_ref = %v; want ghcr.io/agentfm/test:v1", ref)
	}
	if cap := worker["agent_capability"]; cap != "code-helper" {
		t.Errorf("agent_capability = %v; want code-helper", cap)
	}
	if da, ok := worker["dispatch_allowed"].(bool); !ok || !da {
		t.Errorf("dispatch_allowed should be true for a normal worker; got %v", worker["dispatch_allowed"])
	}
	if online, ok := worker["online"].(bool); !ok || !online {
		t.Errorf("online should be true for a live worker; got %v", worker["online"])
	}
}

// TestVisibilityFields_Models asserts that GET /v1/models includes
// the new agentfm_ prefixed visibility fields.
func TestVisibilityFields_Models(t *testing.T) {
	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{
			"peer1": {
				PeerID:           "peer1",
				AgentName:        "test-agent",
				AgentImageRef:    "ghcr.io/agentfm/test:v1",
				AgentImageDigest: "sha256:abc123",
				AgentCapability:  "hr-specialist",
			},
		},
		lastSeen: make(map[string]time.Time),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	b.handleModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	body := rec.Body.String()
	mustHave := []string{
		`"agentfm_image_ref"`,
		`"agentfm_image_digest"`,
		`"agentfm_capability"`,
		`"agentfm_honesty_score"`,
		`"agentfm_is_equivocator"`,
		`"agentfm_dispatch_allowed"`,
		`"agentfm_online"`,
	}
	for _, field := range mustHave {
		if !strings.Contains(body, field) {
			t.Errorf("field %q missing from /v1/models response body", field)
		}
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

package boss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentfm/internal/types"
	"agentfm/test/testutil"
)

func TestGetWorkers_OfflinePeerKeepsCachedName(t *testing.T) {
	b, store := newBossForWorkersTest(t)

	offlineSubj := testutil.NewHost(t)
	subject := offlineSubj.ID().String()
	testutil.AppendOwnRating(t, store, b.HostForTest(), offlineSubj.ID(), -0.2, "test")

	b.lastProfile = map[string]types.WorkerProfile{
		subject: {PeerID: subject, AgentName: "HR Agent (Public)", AgentDesc: "Drafts leave emails", Author: "Anonymous"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workers?include_offline=true", nil)
	rec := httptest.NewRecorder()
	b.handleGetWorkers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Agents []apiWorker `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var found *apiWorker
	for i := range resp.Agents {
		if resp.Agents[i].PeerID == subject {
			found = &resp.Agents[i]
		}
	}
	if found == nil {
		t.Fatalf("subject peer not in response")
	}
	if found.Online {
		t.Errorf("expected offline")
	}
	if found.Name != "HR Agent (Public)" {
		t.Errorf("offline name = %q, want cached 'HR Agent (Public)'", found.Name)
	}
	if found.Description != "Drafts leave emails" {
		t.Errorf("offline description = %q, want cached", found.Description)
	}
}

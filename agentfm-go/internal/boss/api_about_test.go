package boss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAboutEndpoint_ReturnsBackendIdentity(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/about", nil)
	b.handleAboutForTest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	for _, f := range []string{"boss_peer_id", "version", "ledger_tree_size", "reputation_floor", "uptime_seconds"} {
		if _, ok := got[f]; !ok {
			t.Errorf("missing field %q", f)
		}
	}
}

func TestAboutEndpoint_RejectsNonGET(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/about", nil)
	b.handleAboutForTest(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405; got %d", rec.Code)
	}
}

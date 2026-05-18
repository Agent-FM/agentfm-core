package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServesEmbeddedHTML(t *testing.T) {
	h := Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/peer/12D3KooWAbc", nil)
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", got)
	}
	body := rec.Body.String()
	// Sanity checks against the embedded HTML — pick stable strings
	// so cosmetic edits don't break the test.
	for _, marker := range []string{
		"AGENTFM // VERIFIABLE AGENT MESH",
		"/v1/peers/",
		"EQUIVOCATOR",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("response body missing %q", marker)
		}
	}
}

func TestHandler_RejectsNonGET(t *testing.T) {
	h := Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/peer/abc", nil)
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandler_404OnWrongPath(t *testing.T) {
	h := Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/something/else", nil)
	h(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

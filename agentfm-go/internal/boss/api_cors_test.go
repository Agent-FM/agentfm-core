package boss

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// M2: a state-changing request from a cross-site browser origin must be
// refused (CSRF against a token-less loopback boss). The opaque "null" /
// file:// origin is refused unless the host opts in via
// AGENTFM_ALLOW_FILE_ORIGIN (the desktop sets it for its file:// renderer);
// localhost and safe methods are always allowed.
func TestCorsMiddleware_BlocksCrossOriginStateChange(t *testing.T) {
	newH := func(called *bool) http.HandlerFunc {
		return corsMiddleware(func(w http.ResponseWriter, _ *http.Request) {
			*called = true
			w.WriteHeader(http.StatusOK)
		})
	}

	postOrigin := func(origin string) (*httptest.ResponseRecorder, bool) {
		called := false
		req := httptest.NewRequest(http.MethodPost, "/api/execute", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		newH(&called)(rec, req)
		return rec, called
	}

	t.Run("evil-origin POST is 403", func(t *testing.T) {
		rec, called := postOrigin("https://evil.example")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
		if called {
			t.Fatal("handler ran despite cross-origin POST")
		}
	})

	t.Run("null-origin POST refused without opt-in", func(t *testing.T) {
		rec, called := postOrigin("null")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for null origin without opt-in, got %d", rec.Code)
		}
		if called {
			t.Fatal("null-origin POST ran without AGENTFM_ALLOW_FILE_ORIGIN")
		}
	})

	t.Run("null-origin POST allowed with opt-in", func(t *testing.T) {
		t.Setenv("AGENTFM_ALLOW_FILE_ORIGIN", "1")
		if _, called := postOrigin("null"); !called {
			t.Fatal("null-origin POST blocked despite AGENTFM_ALLOW_FILE_ORIGIN=1")
		}
	})

	for _, tc := range []struct {
		name, method, origin string
	}{
		{"localhost POST", http.MethodPost, "http://localhost:5173"},
		{"127.0.0.1 POST", http.MethodPost, "http://127.0.0.1:8080"},
		{"no-origin POST", http.MethodPost, ""},
		{"evil-origin GET (safe)", http.MethodGet, "https://evil.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			req := httptest.NewRequest(tc.method, "/api/execute", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			newH(&called)(rec, req)
			if !called {
				t.Fatalf("handler was blocked but should have run (code %d)", rec.Code)
			}
		})
	}
}

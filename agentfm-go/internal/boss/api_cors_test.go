package boss

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// M2: a state-changing request from a cross-site browser origin must be
// refused (CSRF against a token-less loopback boss), while the desktop
// (file:// → Origin: null), localhost, and safe methods are allowed.
func TestCorsMiddleware_BlocksCrossOriginStateChange(t *testing.T) {
	newH := func(called *bool) http.HandlerFunc {
		return corsMiddleware(func(w http.ResponseWriter, _ *http.Request) {
			*called = true
			w.WriteHeader(http.StatusOK)
		})
	}

	t.Run("evil-origin POST is 403", func(t *testing.T) {
		called := false
		req := httptest.NewRequest(http.MethodPost, "/api/execute", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		newH(&called)(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
		if called {
			t.Fatal("handler ran despite cross-origin POST")
		}
	})

	for _, tc := range []struct {
		name, method, origin string
	}{
		{"desktop null-origin POST", http.MethodPost, "null"},
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

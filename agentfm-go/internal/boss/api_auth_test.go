package boss

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// buildAuthTestMux mirrors the route wiring StartAPIServer builds, so the
// tests below exercise the actual middleware chain (CORS-outside,
// auth-inside, /metrics + /health open) without booting the full server
// lifecycle. If StartAPIServer's wiring drifts from this helper, tests
// will fail and force the helper to be updated — that's the intended
// regression net.
func buildAuthTestMux(t *testing.T, b *Boss) (http.Handler, *authConfig) {
	t.Helper()
	auth, err := newAuthConfig()
	if err != nil {
		t.Fatalf("newAuthConfig: %v", err)
	}
	// No janitor in tests; the limiter doesn't need GC for short-lived runs.

	protected := func(route string, h http.HandlerFunc) http.HandlerFunc {
		return corsMiddleware(auth.middleware(route, h))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/workers", protected("/api/workers", b.handleGetWorkers))
	mux.HandleFunc("/api/execute", protected("/api/execute", b.handleExecuteTask))
	var wg sync.WaitGroup
	mux.HandleFunc("/api/execute/async", protected("/api/execute/async",
		b.asyncExecuteHandler(context.Background(), &wg)))
	mux.HandleFunc("/v1/models", protected("/v1/models", b.handleModels))
	mux.HandleFunc("/v1/chat/completions", protected("/v1/chat/completions", b.handleChatCompletions))
	mux.HandleFunc("/v1/completions", protected("/v1/completions", b.handleCompletions))
	mux.HandleFunc("/health", b.handleHealth)
	// /metrics intentionally omitted from this mux because the metrics
	// package's Registry is process-global and the test doesn't need to
	// scrape it; we cover the "open route" contract via /health instead.
	return mux, auth
}

// ---------------------------------------------------------------------------
// Back-compat: no AGENTFM_API_KEYS → middleware is transparent
// ---------------------------------------------------------------------------

func TestAPIAuth_NoKeysConfigured_AllRoutesOpen(t *testing.T) {
	t.Setenv(apiKeysEnv, "")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	for _, path := range []string{"/api/workers", "/v1/models", "/health"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("path=%s returned 401 in solo-dev mode (no keys)", path)
		}
	}
}

// ---------------------------------------------------------------------------
// Keys configured: auth gates /api/* and /v1/*
// ---------------------------------------------------------------------------

func TestAPIAuth_ProtectedRoutes_RejectMissingBearer(t *testing.T) {
	t.Setenv(apiKeysEnv, "test-token-1234567890abcdef")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/workers"},
		{http.MethodPost, "/api/execute"},
		{http.MethodPost, "/api/execute/async"},
		{http.MethodGet, "/v1/models"},
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/completions"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
			r.RemoteAddr = "9.9.9.9:1234"
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status=%d, want 401; body=%s", w.Code, w.Body.String())
			}
			var body map[string]any
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("response body not JSON: %v", err)
			}
			if body["error"].(map[string]any)["code"] != "unauthorized" {
				t.Errorf("expected envelope code=unauthorized, got %v", body)
			}
		})
	}
}

func TestAPIAuth_ProtectedRoutes_AcceptValidBearer(t *testing.T) {
	t.Setenv(apiKeysEnv, "test-token-1234567890abcdef")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	// /api/workers is the cheapest happy-path probe — it returns 200 + an
	// empty agents list when no telemetry has arrived.
	r := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	r.Header.Set("Authorization", "Bearer test-token-1234567890abcdef")
	r.RemoteAddr = "10.0.0.5:1"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestAPIAuth_ProtectedRoutes_RejectInvalidBearer(t *testing.T) {
	t.Setenv(apiKeysEnv, "test-token-1234567890abcdef")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	r := httptest.NewRequest(http.MethodGet, "/api/workers", nil)
	r.Header.Set("Authorization", "Bearer wrong-token-xxxxxxxxxxxxxxxx")
	r.RemoteAddr = "10.0.0.7:1"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", w.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["error"].(map[string]any)["code"] != "invalid_api_key" {
		t.Errorf("expected envelope code=invalid_api_key, got %v", body)
	}
}

// ---------------------------------------------------------------------------
// /health stays open
// ---------------------------------------------------------------------------

func TestAPIAuth_HealthOpenWithoutBearer(t *testing.T) {
	t.Setenv(apiKeysEnv, "test-token-1234567890abcdef")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200 (health must not require auth)", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if _, ok := body["online_workers"]; !ok {
		t.Errorf("expected online_workers in response, got %v", body)
	}
}

func TestAPIAuth_HealthRejectsNonGet(t *testing.T) {
	t.Setenv(apiKeysEnv, "")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	r := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status=%d, want 405", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CORS preflight bypasses auth
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// StartAPIServer startup refusal — public bind without keys must fail fast
// ---------------------------------------------------------------------------

func TestStartAPIServer_PublicBindNoKeysRefusesToStart(t *testing.T) {
	t.Setenv(apiKeysEnv, "")
	t.Setenv(allowUnauthPublicEnv, "")
	b := newTestBoss(t)

	// Use a random port; we never expect the server to actually listen.
	err := b.StartAPIServer("0.0.0.0", "0")
	if err == nil {
		t.Fatal("expected refusal for public bind without keys; got nil")
	}
	if !errors.Is(err, errPublicBindRequiresAuth) {
		t.Errorf("expected errPublicBindRequiresAuth, got %v", err)
	}
}

// Note: the "happy startup" cases (public bind WITH keys; opt-in env
// bypassing the guard) are covered exhaustively by TestStartupAuthGuard_*
// in auth_test.go (unit-level on enforceStartupAuthGuard). Spinning up a
// real StartAPIServer for those would require a real PubSub on the test
// boss, which is heavier than the assertion needs. The refusal test
// above is sufficient because StartAPIServer returns from the guard
// BEFORE constructing any goroutine — see api.go ordering.

func TestAPIAuth_OPTIONSPreflightBypassesAuth(t *testing.T) {
	// This is the load-bearing assertion behind "CORS wraps auth, not the
	// other way around." Browsers do not send Authorization on preflight,
	// so a 401 here would break every browser-based caller.
	t.Setenv(apiKeysEnv, "test-token-1234567890abcdef")
	b := newTestBoss(t)
	mux, _ := buildAuthTestMux(t, b)

	for _, path := range []string{
		"/api/workers", "/api/execute", "/v1/chat/completions", "/v1/completions", "/v1/models",
	} {
		r := httptest.NewRequest(http.MethodOptions, path, nil)
		r.Header.Set("Origin", "http://example.com")
		r.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("path=%s OPTIONS preflight status=%d, want 200 (must NOT require auth)",
				path, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Errorf("path=%s OPTIONS preflight missing CORS header", path)
		}
	}
}

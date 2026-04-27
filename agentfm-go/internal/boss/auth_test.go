package boss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// loadTokensFromEnv
// ---------------------------------------------------------------------------

func TestLoadTokensFromEnv_EmptyOrUnset(t *testing.T) {
	t.Setenv(apiKeysEnv, "")
	ts, err := loadTokensFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ts.empty() {
		t.Errorf("expected empty token set, got %d tokens", len(ts.tokens))
	}
}

func TestLoadTokensFromEnv_SingleToken(t *testing.T) {
	t.Setenv(apiKeysEnv, "abcdef0123456789")
	ts, err := loadTokensFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts.tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(ts.tokens))
	}
}

func TestLoadTokensFromEnv_MultipleTrimmed(t *testing.T) {
	t.Setenv(apiKeysEnv, "abcdef0123456789 , xxxxxxxxxxxxxxxxxx ,, yyyyyyyyyyyyyyyy")
	ts, err := loadTokensFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts.tokens) != 3 {
		t.Fatalf("expected 3 tokens (empty entry skipped), got %d", len(ts.tokens))
	}
}

func TestLoadTokensFromEnv_DeduplicatesIdenticalTokens(t *testing.T) {
	t.Setenv(apiKeysEnv, "abcdef0123456789,abcdef0123456789")
	ts, _ := loadTokensFromEnv()
	if len(ts.tokens) != 1 {
		t.Errorf("expected dedup to 1 token, got %d", len(ts.tokens))
	}
}

func TestLoadTokensFromEnv_RejectsShortTokens(t *testing.T) {
	t.Setenv(apiKeysEnv, "tooshort")
	_, err := loadTokensFromEnv()
	if err == nil {
		t.Fatal("expected error for token shorter than authMinTokenLength")
	}
	if !strings.Contains(err.Error(), "shorter") {
		t.Errorf("error should mention length: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tokenSet.matches — constant-time semantics
// ---------------------------------------------------------------------------

func TestTokenSet_MatchesAccept(t *testing.T) {
	ts := &tokenSet{tokens: [][]byte{[]byte("alpha-1234567890abc"), []byte("beta-9876543210xyz")}}
	if !ts.matches([]byte("alpha-1234567890abc")) {
		t.Error("expected exact match to accept")
	}
	if !ts.matches([]byte("beta-9876543210xyz")) {
		t.Error("expected second-entry exact match to accept")
	}
}

func TestTokenSet_MismatchReject(t *testing.T) {
	ts := &tokenSet{tokens: [][]byte{[]byte("alpha-1234567890abc")}}
	for _, candidate := range []string{
		"",
		"alpha-1234567890abd",  // one byte off at the end
		"alpha-1234567890ab",   // shorter (different length)
		"alpha-1234567890abcd", // longer
		"ALPHA-1234567890ABC",  // case-different
	} {
		if ts.matches([]byte(candidate)) {
			t.Errorf("expected reject for %q", candidate)
		}
	}
}

func TestTokenSet_EmptySetMatchesNothing(t *testing.T) {
	ts := &tokenSet{}
	if ts.matches([]byte("anything")) {
		t.Error("empty set must match nothing")
	}
	if !ts.empty() {
		t.Error("empty()should return true on a zero-init tokenSet")
	}
}

// ---------------------------------------------------------------------------
// parseBearer
// ---------------------------------------------------------------------------

func TestParseBearer_Valid(t *testing.T) {
	tok, outcome := parseBearer("Bearer abc123")
	if outcome != "" {
		t.Errorf("outcome=%q, want empty", outcome)
	}
	if tok != "abc123" {
		t.Errorf("token=%q, want abc123", tok)
	}
}

func TestParseBearer_CaseInsensitiveScheme(t *testing.T) {
	for _, h := range []string{"bearer abc123", "BEARER abc123", "BeArEr abc123"} {
		tok, outcome := parseBearer(h)
		if outcome != "" || tok != "abc123" {
			t.Errorf("header %q: tok=%q outcome=%q", h, tok, outcome)
		}
	}
}

func TestParseBearer_Missing(t *testing.T) {
	for _, h := range []string{"", "   ", "\t\t"} {
		_, outcome := parseBearer(h)
		if outcome != authOutcomeMissing {
			t.Errorf("header %q: outcome=%q, want missing", h, outcome)
		}
	}
}

func TestParseBearer_Malformed(t *testing.T) {
	for _, h := range []string{
		"Basic dXNlcjpwYXNz", // wrong scheme
		"Bearer ",            // empty token
		"Bearer    ",         // pure whitespace token
		"abc123",             // no scheme prefix
		"Bearertoken",        // no separator space
	} {
		_, outcome := parseBearer(h)
		if outcome != authOutcomeMalformed {
			t.Errorf("header %q: outcome=%q, want malformed", h, outcome)
		}
	}
}

// ---------------------------------------------------------------------------
// badAttemptLimiter
// ---------------------------------------------------------------------------

func TestBadAttemptLimiter_BurstThenBlocks(t *testing.T) {
	l := newBadAttemptLimiter()
	now := time.Now()
	for i := 0; i < authBadAttemptsPerMin; i++ {
		if !l.allow("1.2.3.4", now) {
			t.Fatalf("attempt %d: expected allow within burst", i)
		}
	}
	if l.allow("1.2.3.4", now) {
		t.Error("expected reject after burst exhausted")
	}
}

func TestBadAttemptLimiter_SeparateIPsIndependent(t *testing.T) {
	l := newBadAttemptLimiter()
	now := time.Now()
	for i := 0; i < authBadAttemptsPerMin; i++ {
		l.allow("1.1.1.1", now)
	}
	if !l.allow("2.2.2.2", now) {
		t.Error("second IP should have its own budget")
	}
}

func TestBadAttemptLimiter_RefillsOverTime(t *testing.T) {
	l := newBadAttemptLimiter()
	t0 := time.Now()
	for i := 0; i < authBadAttemptsPerMin; i++ {
		l.allow("3.3.3.3", t0)
	}
	if l.allow("3.3.3.3", t0) {
		t.Fatal("burst should be exhausted")
	}
	// Advance enough wall-clock to refill ~10 tokens.
	if !l.allow("3.3.3.3", t0.Add(20*time.Second)) {
		t.Error("expected refill after 20s")
	}
}

func TestBadAttemptLimiter_EvictsOldestAtCap(t *testing.T) {
	l := newBadAttemptLimiter()
	l.maxKeys = 3
	now := time.Now()
	l.allow("a", now)
	l.allow("b", now.Add(1*time.Second))
	l.allow("c", now.Add(2*time.Second))
	// Trigger eviction by inserting a 4th IP.
	l.allow("d", now.Add(3*time.Second))
	if _, ok := l.buckets["a"]; ok {
		t.Error("oldest entry 'a' should have been evicted")
	}
	if _, ok := l.buckets["d"]; !ok {
		t.Error("newest entry 'd' should be present")
	}
	if len(l.buckets) != 3 {
		t.Errorf("map size=%d, want 3 (cap)", len(l.buckets))
	}
}

func TestBadAttemptLimiter_GcDropsStaleEntries(t *testing.T) {
	l := newBadAttemptLimiter()
	now := time.Now()
	l.allow("stale", now)
	l.allow("fresh", now.Add(authIPEntryTTL+1*time.Minute))
	l.gc(now.Add(authIPEntryTTL + 1*time.Minute))
	if _, ok := l.buckets["stale"]; ok {
		t.Error("stale entry should have been gc'd")
	}
	if _, ok := l.buckets["fresh"]; !ok {
		t.Error("fresh entry should remain")
	}
}

func TestBadAttemptLimiter_JanitorExitsOnCtxCancel(t *testing.T) {
	l := newBadAttemptLimiter()
	ctx, cancel := context.WithCancel(context.Background())
	l.startJanitor(ctx)
	cancel()
	// No assertion; the test passes if the goroutine has exited.
	// Allow a moment for the goroutine to observe ctx.Done().
	time.Sleep(50 * time.Millisecond)
}

func TestBadAttemptLimiter_ConcurrentAccessIsSafe(t *testing.T) {
	// Run under -race to actually exercise this. Without -race the test
	// only proves the code does not deadlock.
	l := newBadAttemptLimiter()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := "10.0.0." + string(rune('0'+i%10))
			now := time.Now()
			for j := 0; j < 100; j++ {
				l.allow(ip, now)
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// enforceStartupAuthGuard
// ---------------------------------------------------------------------------

func TestStartupAuthGuard_LoopbackNoKeysIsFine(t *testing.T) {
	t.Setenv(allowUnauthPublicEnv, "")
	ts := &tokenSet{}
	for _, bind := range []string{"", "127.0.0.1", "::1", "localhost", "127.0.0.42"} {
		if err := enforceStartupAuthGuard(bind, ts); err != nil {
			t.Errorf("bind=%q: unexpected error %v", bind, err)
		}
	}
}

func TestStartupAuthGuard_PublicNoKeysIsFatal(t *testing.T) {
	t.Setenv(allowUnauthPublicEnv, "")
	ts := &tokenSet{}
	for _, bind := range []string{"0.0.0.0", "10.0.0.5", "192.168.1.1", "203.0.113.5"} {
		if err := enforceStartupAuthGuard(bind, ts); err == nil {
			t.Errorf("bind=%q: expected refusal, got nil", bind)
		}
	}
}

func TestStartupAuthGuard_PublicWithKeysIsFine(t *testing.T) {
	ts := &tokenSet{tokens: [][]byte{[]byte("aaaaaaaaaaaaaaaaaaaa")}}
	if err := enforceStartupAuthGuard("0.0.0.0", ts); err != nil {
		t.Errorf("public+keys must succeed; got %v", err)
	}
}

func TestStartupAuthGuard_PublicNoKeysOptInWorks(t *testing.T) {
	t.Setenv(allowUnauthPublicEnv, "1")
	ts := &tokenSet{}
	if err := enforceStartupAuthGuard("0.0.0.0", ts); err != nil {
		t.Errorf("opt-in env should bypass guard; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// middleware end-to-end (no real Boss, no libp2p)
// ---------------------------------------------------------------------------

func TestMiddleware_NoTokensConfiguredIsTransparent(t *testing.T) {
	a := &authConfig{tokens: &tokenSet{}, limiter: newBadAttemptLimiter()}
	called := false
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h(w, r)
	if !called {
		t.Error("handler should have been called when no tokens configured")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
}

func TestMiddleware_ValidTokenAccepts(t *testing.T) {
	a := &authConfig{
		tokens:  &tokenSet{tokens: [][]byte{[]byte("good-token-xxxxxxxxx")}},
		limiter: newBadAttemptLimiter(),
	}
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer good-token-xxxxxxxxx")
	r.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestMiddleware_MissingHeaderRejects401(t *testing.T) {
	a := &authConfig{
		tokens:  &tokenSet{tokens: [][]byte{[]byte("good-token-xxxxxxxxx")}},
		limiter: newBadAttemptLimiter(),
	}
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called on auth failure")
	})
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", w.Code)
	}
	body := readJSONBody(t, w.Body)
	if body["error"].(map[string]any)["code"] != "unauthorized" {
		t.Errorf("expected code=unauthorized, got %v", body)
	}
}

func TestMiddleware_InvalidTokenRejects401WithCorrectCode(t *testing.T) {
	a := &authConfig{
		tokens:  &tokenSet{tokens: [][]byte{[]byte("good-token-xxxxxxxxx")}},
		limiter: newBadAttemptLimiter(),
	}
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called on auth failure")
	})
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer wrong-token-xxxxxxxxxxx")
	r.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", w.Code)
	}
	body := readJSONBody(t, w.Body)
	code := body["error"].(map[string]any)["code"]
	if code != "invalid_api_key" {
		t.Errorf("code=%v, want invalid_api_key", code)
	}
}

func TestMiddleware_RateLimitsAfterBurst(t *testing.T) {
	a := &authConfig{
		tokens:  &tokenSet{tokens: [][]byte{[]byte("good-token-xxxxxxxxx")}},
		limiter: newBadAttemptLimiter(),
	}
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not be called for any of these requests")
	})

	for i := 0; i < authBadAttemptsPerMin; i++ {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("Authorization", "Bearer wrong-xxxxxxxxxxxxxxxxxxxxxxxxx")
		r.RemoteAddr = "5.5.5.5:1"
		w := httptest.NewRecorder()
		h(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status=%d, want 401", i, w.Code)
		}
	}
	// One more attempt: should be rate-limited.
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer wrong-xxxxxxxxxxxxxxxxxxxxxxxxx")
	r.RemoteAddr = "5.5.5.5:1"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst exhaustion; got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 must include Retry-After header")
	}
	body := readJSONBody(t, w.Body)
	code := body["error"].(map[string]any)["code"]
	if code != "rate_limited" {
		t.Errorf("code=%v, want rate_limited", code)
	}
}

func TestMiddleware_MultipleAuthorizationHeadersRejected(t *testing.T) {
	a := &authConfig{
		tokens:  &tokenSet{tokens: [][]byte{[]byte("good-token-xxxxxxxxx")}},
		limiter: newBadAttemptLimiter(),
	}
	h := a.middleware("/test", func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called")
	})
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Add("Authorization", "Bearer good-token-xxxxxxxxx")
	r.Header.Add("Authorization", "Bearer also-valid-but-still-bad")
	r.RemoteAddr = "9.9.9.9:1"
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", w.Code)
	}
}

// readJSONBody is a tiny helper used by middleware tests.
func readJSONBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return m
}

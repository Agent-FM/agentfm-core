// Bearer-token authentication for the Boss HTTP gateway.
//
// Layered defenses:
//   - constant-time token comparison (defeats timing oracles)
//   - bearer parser tolerant of case but not of whitespace inside the token
//   - per-IP failed-attempt rate limiter (token bucket on failures only;
//     successful authenticated requests are NOT rate-limited here)
//   - bounded IP map with janitor goroutine bound to the API server's
//     rootCtx so the limiter cannot leak memory or outlive the server
//
// Token sources (priority, low-to-high):
//  1. AGENTFM_API_KEYS env var — comma-separated tokens
//  2. (future) YAML config file — see Issue #11
//  3. (future) --api-key CLI flag (repeatable) — see Issue #11
//
// Operational modes:
//   - tokens.empty() → middleware is a no-op (back-compat "solo dev" mode).
//     Combined with the loopback-default bind in cmd/agentfm, an unauthenticated
//     gateway is only reachable from 127.0.0.1.
//   - tokens non-empty → /api/* and /v1/* require Authorization: Bearer <tok>.
//     /metrics and /health stay open (Prometheus + LB probes need no creds).
package boss

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/obs"
)

const (
	// Env vars
	apiKeysEnv           = "AGENTFM_API_KEYS"
	allowUnauthPublicEnv = "AGENTFM_ALLOW_UNAUTH_PUBLIC"

	// Rate limiter knobs (per-IP, failures only).
	authBadAttemptsPerMin = 30
	authIPMapMaxEntries   = 4096
	authIPMapJanitorTick  = 1 * time.Minute
	authIPEntryTTL        = 10 * time.Minute

	// Minimum token length. OpenAI keys are 51 chars; we cap defensively
	// at 16 to refuse obviously-weak operator passwords. Acceptable to relax
	// later if a use case appears.
	authMinTokenLength = 16
)

// authOutcome label values for agentfm_auth_attempts_total.
// Mirrored on the Python side as exception envelope codes.
const (
	authOutcomeOK        = "ok"
	authOutcomeMissing   = "missing"
	authOutcomeMalformed = "malformed"
	authOutcomeInvalid   = "invalid"
	authOutcomeRateLimit = "rate_limited"
)

// tokenSet holds the configured bearer tokens as raw bytes. Membership
// tests use crypto/subtle.ConstantTimeCompare and never short-circuit, so
// match position is not observable through timing.
type tokenSet struct {
	tokens [][]byte
}

// loadTokensFromEnv reads AGENTFM_API_KEYS and returns the parsed set.
// Empty env (or unset) yields an empty set (back-compat solo-dev mode).
// Returns an error if any token is shorter than authMinTokenLength so
// operators can't accidentally ship "test" or "1234" as a password.
func loadTokensFromEnv() (*tokenSet, error) {
	raw := strings.TrimSpace(os.Getenv(apiKeysEnv))
	if raw == "" {
		return &tokenSet{}, nil
	}
	parts := strings.Split(raw, ",")
	out := make([][]byte, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if len(t) < authMinTokenLength {
			return nil, fmt.Errorf(
				"auth: token in %s shorter than %d chars (rejected for safety)",
				apiKeysEnv, authMinTokenLength,
			)
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, []byte(t))
	}
	return &tokenSet{tokens: out}, nil
}

// empty reports whether no tokens are configured (solo-dev mode).
func (s *tokenSet) empty() bool { return len(s.tokens) == 0 }

// matches reports whether candidate is in the set, in constant time.
// Iterates every entry regardless of match position.
func (s *tokenSet) matches(candidate []byte) bool {
	matched := 0
	for _, t := range s.tokens {
		if subtle.ConstantTimeCompare(t, candidate) == 1 {
			matched = 1
		}
	}
	return matched == 1
}

// parseBearer extracts the token portion of an Authorization header value.
// Scheme match is case-insensitive (RFC 6750 §2.1: senders SHOULD use
// "Bearer" exactly, but receivers SHOULD accept any case). Token bytes
// are returned verbatim (not trimmed) — bearer tokens may contain padding
// that is part of the credential.
func parseBearer(header string) (token, badOutcome string) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", authOutcomeMissing
	}
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", authOutcomeMalformed
	}
	t := header[len(prefix):]
	// Reject a token that is empty or pure whitespace.
	if strings.TrimSpace(t) == "" {
		return "", authOutcomeMalformed
	}
	return t, ""
}

// errPublicBindRequiresAuth is returned by enforceStartupAuthGuard when
// the operator binds the gateway off-loopback without configured tokens.
var errPublicBindRequiresAuth = errors.New(
	"refusing to start: bind is non-loopback and no API keys configured. " +
		"Set AGENTFM_API_KEYS=key1,key2,... or AGENTFM_ALLOW_UNAUTH_PUBLIC=1 to opt in",
)

// enforceStartupAuthGuard returns an error if the gateway would bind to a
// non-loopback interface with no auth configured AND the operator has not
// explicitly opted in via AGENTFM_ALLOW_UNAUTH_PUBLIC=1.
//
// Rationale: a public unauthenticated gateway is a Podman-time-share faucet.
// We want operators to make an explicit choice rather than discover the
// exposure through their bill or their abuse desk.
func enforceStartupAuthGuard(bind string, tokens *tokenSet) error {
	if !tokens.empty() {
		return nil
	}
	if isLoopbackBind(bind) {
		return nil
	}
	if os.Getenv(allowUnauthPublicEnv) == "1" {
		return nil
	}
	return errPublicBindRequiresAuth
}

// isLoopbackBind classifies a host:port-less bind string as loopback or not.
// Treats empty / "localhost" / "127.0.0.1" / "::1" as loopback. Rejects
// "0.0.0.0" and any non-loopback IP literal.
func isLoopbackBind(bind string) bool {
	switch strings.ToLower(strings.TrimSpace(bind)) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(bind); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// ---------------------------------------------------------------------------
// Per-IP rate limiter — token bucket on FAILURES only.
//
// Successful authenticated requests are not metered here (per-key request
// quotas are deferred to v2 per Issue #13). The limiter exists to slow
// brute-force / spray attacks, not to throttle legitimate users.
// ---------------------------------------------------------------------------

type ipBucket struct {
	tokens   float64
	lastSeen time.Time
}

type badAttemptLimiter struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
	rate    float64 // failures restored per second
	burst   float64 // bucket capacity
	maxKeys int
}

func newBadAttemptLimiter() *badAttemptLimiter {
	return &badAttemptLimiter{
		buckets: make(map[string]*ipBucket, 256),
		rate:    float64(authBadAttemptsPerMin) / 60.0,
		burst:   float64(authBadAttemptsPerMin),
		maxKeys: authIPMapMaxEntries,
	}
}

// allow returns true iff the given IP may consume one failure budget unit.
// Always called BEFORE serving the failure response, so the per-IP cost of
// each rejection is recorded even when the response itself is 401.
func (l *badAttemptLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.evictOldestLocked()
		}
		b = &ipBucket{tokens: l.burst, lastSeen: now}
		l.buckets[ip] = b
	}
	delta := now.Sub(b.lastSeen).Seconds() * l.rate
	if t := b.tokens + delta; t < l.burst {
		b.tokens = t
	} else {
		b.tokens = l.burst
	}
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evictOldestLocked drops the single least-recently-seen entry. Caller must
// hold l.mu. O(n) over the map; only called when the map is at maxKeys
// (rare under normal load).
func (l *badAttemptLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestSeen time.Time
	first := true
	for ip, b := range l.buckets {
		if first || b.lastSeen.Before(oldestSeen) {
			oldestIP = ip
			oldestSeen = b.lastSeen
			first = false
		}
	}
	delete(l.buckets, oldestIP)
}

// startJanitor runs a background goroutine that GCs cold entries on a
// fixed cadence. The goroutine exits when ctx is cancelled — the caller
// (StartAPIServer) supplies the same ctx that bounds every other server
// goroutine, so shutdown ordering is automatic.
func (l *badAttemptLimiter) startJanitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(authIPMapJanitorTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				l.gc(now)
			}
		}
	}()
}

// gc removes entries last seen more than authIPEntryTTL ago.
func (l *badAttemptLimiter) gc(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, b := range l.buckets {
		if now.Sub(b.lastSeen) > authIPEntryTTL {
			delete(l.buckets, ip)
		}
	}
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// authConfig groups the immutable token set and the mutable rate limiter.
// One instance per StartAPIServer invocation.
type authConfig struct {
	tokens  *tokenSet
	limiter *badAttemptLimiter
}

// newAuthConfig loads tokens from env and constructs a limiter. The
// limiter's janitor is started by the caller via cfg.limiter.startJanitor(ctx).
func newAuthConfig() (*authConfig, error) {
	tokens, err := loadTokensFromEnv()
	if err != nil {
		return nil, err
	}
	return &authConfig{tokens: tokens, limiter: newBadAttemptLimiter()}, nil
}

// middleware returns an http.HandlerFunc that gates `next` on a valid
// Bearer token. When tokens.empty() it is a transparent pass-through
// (solo-dev mode). The route argument is recorded in slog audit lines and
// in the failure-response envelope for operator visibility.
//
// Wrap CORS OUTSIDE this middleware so that OPTIONS preflights pass
// without Authorization (browsers do not send credentials on preflight).
// corsMiddleware short-circuits OPTIONS before calling its `next`.
func (a *authConfig) middleware(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.tokens.empty() {
			next.ServeHTTP(w, r)
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}

		// Authorization is NOT a list-typed header (RFC 7230 §3.2.2).
		// Multiple values is malformed, not "merge them."
		if vs := r.Header.Values("Authorization"); len(vs) > 1 {
			a.fail(w, ip, route, authOutcomeMalformed, "multiple Authorization headers")
			return
		}

		tok, badOutcome := parseBearer(r.Header.Get("Authorization"))
		if badOutcome != "" {
			a.fail(w, ip, route, badOutcome, "missing or malformed Bearer token")
			return
		}
		if !a.tokens.matches([]byte(tok)) {
			a.fail(w, ip, route, authOutcomeInvalid, "invalid API key")
			return
		}

		metrics.AuthAttemptsTotal.WithLabelValues(authOutcomeOK).Inc()
		slog.Debug("auth ok",
			slog.String(obs.FieldRemoteAddr, ip),
			slog.String(obs.FieldRoute, route),
		)
		next.ServeHTTP(w, r)
	}
}

// fail writes the auth-failure response and increments metrics. If the IP
// has exhausted its bad-attempt budget the response is upgraded to 429
// with a Retry-After header.
func (a *authConfig) fail(w http.ResponseWriter, ip, route, outcome, msg string) {
	status := http.StatusUnauthorized
	if !a.limiter.allow(ip, time.Now()) {
		outcome = authOutcomeRateLimit
		status = http.StatusTooManyRequests
		msg = "too many failed auth attempts; slow down"
		w.Header().Set("Retry-After", "60")
	}

	metrics.AuthAttemptsTotal.WithLabelValues(outcome).Inc()
	slog.Warn("auth failure",
		slog.String(obs.FieldRemoteAddr, ip),
		slog.String(obs.FieldRoute, route),
		slog.String(obs.FieldAuthOutcome, outcome),
	)

	// Use the OpenAI envelope universally for auth failures. The legacy
	// /api/* clients accept JSON; the Python SDK's from_envelope handles
	// both shapes (and falls back to GatewayProtocolError for non-envelope
	// bodies anyway).
	writeOpenAIError(w, status, errTypeInvalidRequest, outcomeToCode(outcome), msg)
}

// outcomeToCode maps an internal authOutcome label to the public envelope
// code surfaced in the response body. Public codes are the contract the
// Python SDK matches against in its _CODE_MAP.
func outcomeToCode(outcome string) string {
	switch outcome {
	case authOutcomeRateLimit:
		return "rate_limited"
	case authOutcomeInvalid:
		return "invalid_api_key"
	default: // missing, malformed, anything else
		return "unauthorized"
	}
}

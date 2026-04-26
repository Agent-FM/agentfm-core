package boss

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateWebhookURL_AcceptsPublic(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "")
	// Use literal public IPs so the test does not depend on CI DNS — the
	// validator now fails closed on resolve errors.
	for _, u := range []string{
		"http://93.184.216.34/cb",                        // public IPv4 (example.com)
		"http://[2606:2800:220:1:248:1893:25c8:1946]/cb", // public IPv6
		"https://93.184.216.34:9000/cb",
	} {
		if err := validateWebhookURL(u); err != nil {
			t.Errorf("validateWebhookURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateWebhookURL_FailsClosedOnDNSError(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "")
	// A hostname that's guaranteed not to resolve — the .invalid TLD is
	// reserved for exactly this purpose by RFC 2606.
	if err := validateWebhookURL("http://nonexistent-host.invalid/cb"); err == nil {
		t.Errorf("expected rejection for unresolvable host (fail-closed)")
	}
}

func TestValidateWebhookURL_RejectsBadScheme(t *testing.T) {
	for _, u := range []string{
		"file:///etc/passwd",
		"ftp://example.com/",
		"javascript:alert(1)",
		"",
		"not a url at all", // url.Parse accepts this; host check rejects it
	} {
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("validateWebhookURL(%q) = nil, want error", u)
		}
	}
}

func TestValidateWebhookURL_RejectsLoopbackByDefault(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "")
	for _, u := range []string{
		"http://127.0.0.1/cb",
		"http://[::1]/cb",
		"http://localhost/cb",
		"http://10.0.0.5/cb",
		"http://192.168.1.1/cb",
		"http://169.254.169.254/cb", // EC2 metadata service — classic SSRF target
		"http://172.16.0.1/cb",
		"http://0.0.0.0/cb",
	} {
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("validateWebhookURL(%q) accepted private host; want rejection", u)
		}
	}
}

func TestValidateWebhookURL_AllowsLoopbackWhenOptedIn(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "1")
	if err := validateWebhookURL("http://127.0.0.1/cb"); err != nil {
		t.Errorf("with %s=1, loopback should be accepted; got %v",
			webhookAllowPrivateEnv, err)
	}
}

func TestSignWebhookBody_EmptyWhenNoSecret(t *testing.T) {
	t.Setenv(webhookSecretEnv, "")
	if got := signWebhookBody([]byte(`{"x":1}`)); got != "" {
		t.Errorf("expected empty signature when no secret set, got %q", got)
	}
}

// TestSafeWebhookClient_RefusesPrivateAtDial closes the SSRF TOCTOU bypass.
// We can't easily simulate a DNS-rebinding attack from within Go's resolver,
// but we can exercise the equivalent code path: a hostname that resolves to
// a private IP must be refused at dial even if the validator was somehow
// bypassed. The validator and the dial-side guard are belt-and-braces.
func TestSafeWebhookClient_RefusesPrivateAtDial(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "")
	client := safeWebhookClient(2 * time.Second)
	// localhost resolves to 127.0.0.1 → private. Dial must fail.
	resp, err := client.Get("http://localhost:0/cb")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("expected dial to refuse localhost, got nil error")
	}
	if !strings.Contains(err.Error(), "private/loopback") {
		t.Errorf("expected private/loopback error, got %v", err)
	}
}

func TestSafeWebhookClient_AllowsPrivateWhenOptedIn(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "1")
	client := safeWebhookClient(2 * time.Second)
	// Same call as above but with the opt-in env. The dial may still fail
	// because the port is closed, but it must NOT fail with our SSRF guard.
	resp, err := client.Get("http://127.0.0.1:0/cb")
	if err != nil && strings.Contains(err.Error(), "private/loopback") {
		t.Errorf("with %s=1, private dial should NOT trigger SSRF guard; got %v",
			webhookAllowPrivateEnv, err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
}

// TestSafeWebhookClient_DoesNotFollowRedirects: a 30x → file:// or 30x →
// 169.254.169.254 must NOT be followed. CheckRedirect = ErrUseLastResponse
// stops at the first redirect; the boss treats it as the final response.
func TestSafeWebhookClient_DoesNotFollowRedirects(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "1") // need loopback for httptest

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		// If the SDK followed the redirect we'd see /redirected hit.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := safeWebhookClient(2 * time.Second)
	resp, err := client.Get(srv.URL + "/start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status=%d, want 302 (redirect must NOT be followed)", resp.StatusCode)
	}
	if hits != 1 {
		t.Errorf("server saw %d hits; the SDK followed the redirect (want 1)", hits)
	}
}

// TestMaxWebhookResponseBytes_BoundsBodyRead: a hostile webhook server
// returning a 1 MiB body must not push >MaxWebhookResponseBytes into the
// boss process. We exercise the io.CopyN cap by checking the constant
// itself; the integration is too coupled to runAsyncTask's full path to
// unit test cleanly without a worker stub. The constant + a code-grep is
// the minimum viable pin.
func TestMaxWebhookResponseBytes_HasReasonableBound(t *testing.T) {
	if MaxWebhookResponseBytes <= 0 || MaxWebhookResponseBytes > 1024*1024 {
		t.Errorf("MaxWebhookResponseBytes=%d outside sensible range (>0 and <=1MiB)",
			MaxWebhookResponseBytes)
	}
}

func TestSignWebhookBody_HMACSha256Hex(t *testing.T) {
	secret := "very-secret-key"
	body := []byte(`{"task_id":"t1","status":"completed"}`)
	t.Setenv(webhookSecretEnv, secret)

	got := signWebhookBody(body)

	// Independent computation: a receiver must be able to validate by doing
	// the same calculation. Pinning this here means future-me can't
	// accidentally swap the algorithm without breaking compat.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("signWebhookBody() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "") || len(got) != 64 {
		t.Errorf("expected 64-char hex sha256, got %d chars: %q", len(got), got)
	}
}

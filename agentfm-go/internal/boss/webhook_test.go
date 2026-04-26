package boss

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestValidateWebhookURL_AcceptsPublic(t *testing.T) {
	t.Setenv(webhookAllowPrivateEnv, "")
	for _, u := range []string{
		"https://hooks.example.com/cb",
		"http://example.com:9000/cb",
	} {
		if err := validateWebhookURL(u); err != nil {
			t.Errorf("validateWebhookURL(%q) = %v, want nil", u, err)
		}
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

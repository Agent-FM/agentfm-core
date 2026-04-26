package boss

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// webhookSecretEnv is the environment variable the operator sets to enable
// HMAC signing of outbound webhook POSTs. The Python-side WebhookReceiver
// must be configured with the same secret to verify the signature.
const webhookSecretEnv = "AGENTFM_WEBHOOK_SECRET"

// webhookAllowPrivateEnv lets the operator opt back in to private/loopback
// webhook destinations. Default-deny exists to prevent SSRF against the
// boss-api host's internal network from an off-host SDK client.
const webhookAllowPrivateEnv = "AGENTFM_WEBHOOK_ALLOW_PRIVATE"

// signatureHeader is the HTTP header the boss writes when a secret is set.
// Naming mirrors the convention used by GitHub, Stripe, etc., so receivers
// can recognise it without consulting AgentFM-specific docs.
const signatureHeader = "X-AgentFM-Signature"

// validateWebhookURL refuses URLs that:
//   - aren't http or https
//   - point at loopback / link-local / RFC1918 networks (unless opted in)
//
// Validation runs before the dial so a hostile or misconfigured SDK client
// can't trick the boss-api into scanning its own internal network.
func validateWebhookURL(raw string) error {
	if raw == "" {
		return errors.New("empty webhook URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse webhook URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook scheme %q not allowed (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("webhook URL has empty host")
	}
	if os.Getenv(webhookAllowPrivateEnv) == "1" {
		return nil
	}
	host := u.Hostname()
	if isPrivateOrLoopbackHost(host) {
		return fmt.Errorf("webhook host %q is private/loopback (set %s=1 to allow)",
			host, webhookAllowPrivateEnv)
	}
	return nil
}

// isPrivateOrLoopbackHost returns true if `host` is a literal IP that's
// loopback, link-local, or in an RFC1918 / RFC4193 / IPv6 ULA range, OR if
// it resolves to such an address. We resolve so attackers can't bypass via
// internal DNS pointing example.com → 10.0.0.5.
func isPrivateOrLoopbackHost(host string) bool {
	// Strip brackets from IPv6 literals.
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateOrLoopbackIP(ip)
	}
	// Best-effort lookup. If resolution fails, fail open (the dial will
	// fail later on its own merits). We don't want to block a legitimate
	// webhook just because DNS is briefly flaky.
	addrs, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range addrs {
		if isPrivateOrLoopbackIP(ip) {
			return true
		}
	}
	return false
}

func isPrivateOrLoopbackIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified()
}

// signWebhookBody returns the hex HMAC-SHA256 of body with the configured
// secret, or "" if no secret is set. Receivers compare in constant time
// against their own computed digest.
func signWebhookBody(body []byte) string {
	secret := os.Getenv(webhookSecretEnv)
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

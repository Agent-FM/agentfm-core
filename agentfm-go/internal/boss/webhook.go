package boss

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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
//
// Fails closed on DNS error: a host that won't resolve is treated as
// suspicious. Fail-open SSRF guards are an antipattern (an attacker who
// controls DNS can make resolution fail, then `http.Client.Do` would do its
// own resolution against a different resolver and bypass us).
func isPrivateOrLoopbackHost(host string) bool {
	// Strip brackets from IPv6 literals.
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateOrLoopbackIP(ip)
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		// Fail closed: DNS error is treated as "private/loopback" so the
		// caller refuses the URL.
		return true
	}
	if len(addrs) == 0 {
		return true
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

// safeWebhookClient returns an http.Client whose DialContext re-validates
// every resolved IP before connect. Closes the SSRF TOCTOU bypass where
// validateWebhookURL resolves at validation time and http.Client.Do
// resolves again at dial time — an attacker can return a public IP at
// validation and a private one at dial. Without this DialContext the
// validator would be bypass-able by anyone who controls DNS for a hostname.
func safeWebhookClient(timeout time.Duration) *http.Client {
	if os.Getenv(webhookAllowPrivateEnv) == "1" {
		return &http.Client{Timeout: timeout}
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				// Resolve and check every candidate before allowing the
				// dial. http.Client picks the first that connects, so all
				// candidates are reachable by the dialer — refuse if any
				// is private / loopback.
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, fmt.Errorf("dial %s: resolve %q: %w", network, host, err)
				}
				if len(ips) == 0 {
					return nil, fmt.Errorf("dial %s: no addresses for %q", network, host)
				}
				for _, ip := range ips {
					if isPrivateOrLoopbackIP(ip.IP) {
						return nil, fmt.Errorf(
							"dial %s: refusing to connect to private/loopback %s (resolved from %q)",
							network, ip.IP, host,
						)
					}
				}
				// Dial the first verified address by IP literal so the
				// kernel doesn't re-resolve.
				dialAddr := net.JoinHostPort(ips[0].IP.String(), port)
				return dialer.DialContext(ctx, network, dialAddr)
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
	}
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

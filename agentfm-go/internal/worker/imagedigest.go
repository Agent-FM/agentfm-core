package worker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"agentfm/internal/obs"
)

// ImageDigestResolver resolves an OCI image reference to its
// `sha256:...` manifest digest by shelling out to `podman image
// inspect`. The result is cached per process — image digests are
// immutable for a given pulled image, so one lookup at startup is
// enough.
//
// When podman is unavailable (e.g. boss-only nodes) or the image is
// not present locally, ResolveDigest returns an empty string and a
// nil error — the worker proceeds without a digest and telemetry
// publishes an "unattested" empty field. Boss in strict mode will
// refuse to dispatch to such peers; in warn mode it logs and
// continues.
type ImageDigestResolver struct {
	mu    sync.Mutex
	cache map[string]string // image_ref -> digest
}

// NewImageDigestResolver returns a resolver with an empty cache.
func NewImageDigestResolver() *ImageDigestResolver {
	return &ImageDigestResolver{cache: make(map[string]string)}
}

// ResolveDigest returns the OCI manifest digest of imageRef. Returns
// ("", nil) when the image isn't present locally OR podman isn't on
// PATH — both are non-fatal "unattested" outcomes. Returns ("", err)
// only on a genuine system error (e.g. podman returns garbage).
//
// The lookup runs `podman image inspect --format '{{.Digest}}' <ref>`
// which is what production worker setups already invoke during
// sandbox prep — adding the digest read here piggybacks on the
// same authoritative source.
func (r *ImageDigestResolver) ResolveDigest(ctx context.Context, imageRef string) (string, error) {
	if imageRef == "" {
		return "", nil
	}
	r.mu.Lock()
	if cached, ok := r.cache[imageRef]; ok {
		r.mu.Unlock()
		return cached, nil
	}
	r.mu.Unlock()

	// 5s cap — `podman image inspect` is local-only and should
	// return in well under a second; the timeout protects against a
	// jammed Podman daemon.
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, "podman", "image", "inspect",
		"--format", "{{.Digest}}", imageRef)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// podman not installed → exec.ErrNotFound surfaces here.
		// image not present locally → podman exits non-zero with a
		// stderr like "Error: <ref>: image not known". Both are
		// non-fatal: we proceed unattested.
		stderrText := strings.TrimSpace(stderr.String())
		slog.Debug("podman image inspect did not return a digest; running unattested",
			slog.String("image_ref", imageRef),
			slog.String("stderr", stderrText),
			slog.Any(obs.FieldErr, err))
		return "", nil
	}

	digest := strings.TrimSpace(stdout.String())
	if digest == "" {
		return "", nil
	}
	if !strings.HasPrefix(digest, "sha256:") {
		// Unexpected — podman always emits "sha256:..." today; if
		// the format changes in a future version we treat it as
		// system-bug-worthy and surface the error so an operator
		// can investigate.
		return "", fmt.Errorf("worker: podman returned unexpected digest format %q", digest)
	}

	r.mu.Lock()
	r.cache[imageRef] = digest
	r.mu.Unlock()
	return digest, nil
}

// KebabCapability normalises a free-text capability/agent name into
// a kebab-case tag (e.g. "HR Specialist" → "hr-specialist"). Used as
// the default for AgentCapability when the operator didn't supply
// --capability explicitly.
func KebabCapability(s string) string {
	out := make([]byte, 0, len(s))
	prevDash := true // suppress leading dashes
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, byte(r+('a'-'A')))
			prevDash = false
		case r >= 'a' && r <= 'z':
			out = append(out, byte(r))
			prevDash = false
		case r >= '0' && r <= '9':
			out = append(out, byte(r))
			prevDash = false
		default:
			if !prevDash {
				out = append(out, '-')
				prevDash = true
			}
		}
	}
	// trim trailing dash
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return string(out)
}

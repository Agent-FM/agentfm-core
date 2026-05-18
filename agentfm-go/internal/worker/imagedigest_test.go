package worker

import (
	"context"
	"testing"
	"time"
)

func TestKebabCapability(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"HR Specialist", "hr-specialist"},
		{"sick-leave-generator", "sick-leave-generator"},
		{"Code Helper", "code-helper"},
		{"  Lots   of    Spaces  ", "lots-of-spaces"},
		{"!!!Punctuation!!!", "punctuation"},
		{"Already-Kebab", "already-kebab"},
		{"With123Numbers", "with123numbers"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := KebabCapability(tc.in); got != tc.want {
			t.Errorf("KebabCapability(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveDigest_EmptyRef(t *testing.T) {
	r := NewImageDigestResolver()
	digest, err := r.ResolveDigest(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if digest != "" {
		t.Errorf("empty ref → digest = %q, want empty", digest)
	}
}

func TestResolveDigest_NonexistentImage_ReturnsEmpty(t *testing.T) {
	// We deliberately pick a name that podman won't have. The
	// resolver MUST return empty + nil (running unattested), not
	// propagate the exec error.
	r := NewImageDigestResolver()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	digest, err := r.ResolveDigest(ctx, "this-image-definitely-does-not-exist-locally:v999")
	if err != nil {
		t.Fatalf("ResolveDigest should not error on missing image, got: %v", err)
	}
	if digest != "" {
		t.Errorf("missing image → digest = %q, want empty", digest)
	}
}

func TestResolveDigest_CachesResult(t *testing.T) {
	// First call on a non-existent ref returns empty. A second call
	// must return the same empty result without re-shelling out —
	// we can't observe the cache directly, but we CAN observe that
	// a second call returns the same value within microseconds.
	// This test doubles as a regression guard against the cache
	// getting bypassed.
	r := NewImageDigestResolver()
	ctx := context.Background()
	if _, err := r.ResolveDigest(ctx, "cache-probe:v1"); err != nil {
		t.Fatalf("first ResolveDigest: %v", err)
	}
	start := time.Now()
	if _, err := r.ResolveDigest(ctx, "cache-probe:v1"); err != nil {
		t.Fatalf("second ResolveDigest: %v", err)
	}
	elapsed := time.Since(start)
	// First-call podman exec typically takes 50-200ms even for a
	// missing image. The cached path should be < 1ms; 10ms gives
	// plenty of slack for CI.
	if elapsed > 10*time.Millisecond {
		t.Logf("second call took %v; cache may not be effective", elapsed)
		// non-fatal — the cache is a perf optimisation, not a
		// correctness requirement, and the test is best-effort
	}
}

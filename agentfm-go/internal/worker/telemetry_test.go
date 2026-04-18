package worker

import (
	"testing"

	"agentfm/test/testutil"
)

// TestTruncateWords is a table-driven sweep over the whitespace edge cases
// that trip up naïve string.Split approaches. The method is called on every
// telemetry tick so any regression would immediately pollute the radar UI.
func TestTruncateWords(t *testing.T) {
	t.Parallel()
	w := &Worker{}

	cases := []struct {
		name     string
		text     string
		maxWords int
		want     string
	}{
		{"empty", "", 5, ""},
		{"below limit", "one two three", 5, "one two three"},
		{"at exact limit", "one two three", 3, "one two three"},
		{"above limit", "one two three four five six", 3, "one two three..."},
		{"single word trim", "one two three", 1, "one..."},
		{"collapses multi-space", "one   two   three", 2, "one two..."},
		{"leading/trailing whitespace", "  one two three  ", 2, "one two..."},
		{"maxWords zero", "one two three", 0, "..."},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := w.truncateWords(tc.text, tc.maxWords)
			if got != tc.want {
				t.Errorf("truncateWords(%q, %d) = %q, want %q", tc.text, tc.maxWords, got, tc.want)
			}
		})
	}
}

// --- getGPUStats -----------------------------------------------------------
//
// getGPUStats shells out to `nvidia-smi` via PATH. The tests below install
// a fake nvidia-smi shell script (via testutil.InstallFakeNvidiaSmi), then
// assert how the parsing logic handles each observable variant. POSIX-only;
// testutil.RequirePOSIX skips them on Windows.

// TestGetGPUStats_NvidiaSmiMissing: on a machine without nvidia-smi,
// getGPUStats must report "no GPU" rather than panic — the common case for
// developers running AgentFM on a laptop.
func TestGetGPUStats_NvidiaSmiMissing(t *testing.T) {
	testutil.RequirePOSIX(t)
	t.Setenv("PATH", t.TempDir()) // empty dir on PATH

	hasGPU, used, total, pct := getGPUStats()
	if hasGPU {
		t.Errorf("expected hasGPU=false, got hasGPU=true (used=%f total=%f pct=%f)", used, total, pct)
	}
}

// TestGetGPUStats_NvidiaSmiErrorExit: nvidia-smi exists but exits non-zero
// (e.g. driver not loaded). Function must fall back to "no GPU".
func TestGetGPUStats_NvidiaSmiErrorExit(t *testing.T) {
	testutil.InstallFakeNvidiaSmi(t, "", 1)

	hasGPU, _, _, _ := getGPUStats()
	if hasGPU {
		t.Error("expected hasGPU=false on non-zero exit")
	}
}

// TestGetGPUStats_ValidOutput: the happy path. nvidia-smi returns
// "used_mb, total_mb" — we verify the MB→GB conversion and percentage math.
func TestGetGPUStats_ValidOutput(t *testing.T) {
	testutil.InstallFakeNvidiaSmi(t, "4096, 24576", 0)

	hasGPU, used, total, pct := getGPUStats()
	if !hasGPU {
		t.Fatal("expected hasGPU=true on valid output")
	}
	if used != 4.0 {
		t.Errorf("used = %f, want 4.0", used)
	}
	if total != 24.0 {
		t.Errorf("total = %f, want 24.0", total)
	}
	// 4 / 24 = 16.666...
	if pct < 16 || pct > 17 {
		t.Errorf("pct = %f, want ~16.67", pct)
	}
}

// TestGetGPUStats_MalformedOutput: regression guard for the strconv.ParseFloat
// fix. If nvidia-smi returns garbage we must report "no GPU" instead of
// publishing a zero-for-zero division into the telemetry profile.
func TestGetGPUStats_MalformedOutput(t *testing.T) {
	testutil.InstallFakeNvidiaSmi(t, "not,numbers", 0)

	hasGPU, used, total, pct := getGPUStats()
	if hasGPU {
		t.Errorf("expected hasGPU=false on malformed output, got used=%f total=%f pct=%f", used, total, pct)
	}
}

// TestGetGPUStats_TooFewFields: nvidia-smi returns only one field. The
// len(parts) < 2 branch must short-circuit to "no GPU".
func TestGetGPUStats_TooFewFields(t *testing.T) {
	testutil.InstallFakeNvidiaSmi(t, "4096", 0)

	hasGPU, _, _, _ := getGPUStats()
	if hasGPU {
		t.Error("expected hasGPU=false when nvidia-smi returns a single field")
	}
}

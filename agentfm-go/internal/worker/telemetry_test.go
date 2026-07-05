package worker

import (
	"testing"

	"agentfm/test/testutil"
)

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

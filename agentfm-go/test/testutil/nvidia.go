package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// RequirePOSIX skips the test on Windows, used by helpers that rely on
// /bin/sh to install fake binaries on PATH.
func RequirePOSIX(t testing.TB) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping: helper relies on POSIX shell")
	}
}

// InstallFakeNvidiaSmi drops a shell script named "nvidia-smi" into a fresh
// temp dir, points PATH at it (via t.Setenv so it's restored on cleanup),
// and returns. stdout becomes the script's output; exitCode is the exit
// status. Used by telemetry tests to validate getGPUStats's parsing logic
// without needing a real CUDA GPU.
func InstallFakeNvidiaSmi(t testing.TB, stdout string, exitCode int) {
	t.Helper()
	RequirePOSIX(t)

	binDir := t.TempDir()
	script := filepath.Join(binDir, "nvidia-smi")

	body := "#!/bin/sh\n"
	if stdout != "" {
		// Single-quote the stdout so shell doesn't interpret special chars.
		body += "printf '%s' '" + stdout + "'\n"
	}
	if exitCode != 0 {
		body += "exit " + strconv.Itoa(exitCode) + "\n"
	}

	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake nvidia-smi: %v", err)
	}
	t.Setenv("PATH", binDir)
}

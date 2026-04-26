package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// InstallFakePodman drops a shell script named "podman" into a fresh temp
// dir and points PATH at it (via t.Setenv so it's restored on cleanup).
// The script body is taken verbatim from `body`. Used by sandbox tests to
// exercise control flow (cancellation, timeouts, exit codes) without
// requiring a real Podman install.
//
// Example: a podman that sleeps forever, exercising ctx-cancel:
//
//	testutil.InstallFakePodman(t, "#!/bin/sh\nsleep 60\n")
func InstallFakePodman(t testing.TB, body string) {
	t.Helper()
	RequirePOSIX(t)

	binDir := t.TempDir()
	script := filepath.Join(binDir, "podman")
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
	t.Setenv("PATH", binDir)
}

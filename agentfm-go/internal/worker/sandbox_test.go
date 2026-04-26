package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/test/testutil"
)

// TestBuildSandboxImage_CancelledCtxKillsBuild proves buildSandboxImage is
// killable while a hung `podman build` is in flight. Pre-fix, the call used
// exec.Command which ignored ctx; cancelling the context did nothing and the
// caller had to wait for the process to finish on its own.
func TestBuildSandboxImage_CancelledCtxKillsBuild(t *testing.T) {
	testutil.RequirePOSIX(t)

	// Stub podman: sleep "forever" so the only way out is ctx-cancel → SIGKILL.
	testutil.InstallFakePodman(t, "#!/bin/sh\nsleep 30\n")

	// A directory that has a Containerfile so the precondition check passes.
	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "Containerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write Containerfile: %v", err)
	}

	w := &Worker{config: Config{
		AgentDir:  agentDir,
		ImageName: "agentfm-test",
	}}

	ctx, cancel := context.WithCancel(context.Background())
	// Kill ctx after 200ms — well under the fake podman's 30s sleep.
	time.AfterFunc(200*time.Millisecond, cancel)

	start := time.Now()
	err := w.buildSandboxImage(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error from cancelled build, got nil")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("build did not respond to ctx cancel within budget; took %v", elapsed)
	}
	// Surface the wrap behaviour: %w should preserve the underlying ExitError /
	// signal info. We don't assert the exact type because Linux/macOS differ on
	// SIGKILL surface, but the caller must get *some* error.
	if !strings.Contains(err.Error(), "podman build failed") {
		t.Errorf("expected wrapped 'podman build failed' error, got %v", err)
	}
}

// TestBuildSandboxImage_RejectsMissingDockerfile pins the precondition check.
func TestBuildSandboxImage_RejectsMissingDockerfile(t *testing.T) {
	testutil.RequirePOSIX(t)
	testutil.InstallFakePodman(t, "#!/bin/sh\nexit 0\n")

	w := &Worker{config: Config{
		AgentDir:  t.TempDir(),
		ImageName: "agentfm-test",
	}}

	err := w.buildSandboxImage(context.Background())
	if err == nil {
		t.Fatal("expected error when no Dockerfile/Containerfile present")
	}
	if !strings.Contains(err.Error(), "no Dockerfile") {
		t.Errorf("error %q should mention missing Dockerfile", err)
	}
}

// TestBuildSandboxImage_HappyPath_SuccessfulBuild proves the success branch
// returns nil when podman exits 0.
func TestBuildSandboxImage_HappyPath_SuccessfulBuild(t *testing.T) {
	testutil.RequirePOSIX(t)
	testutil.InstallFakePodman(t, "#!/bin/sh\nexit 0\n")

	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	w := &Worker{config: Config{
		AgentDir:  agentDir,
		ImageName: "agentfm-test",
	}}

	if err := w.buildSandboxImage(context.Background()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

// TestBuildSandboxImage_NonZeroExit pins the failure-to-build error wrap.
func TestBuildSandboxImage_NonZeroExit(t *testing.T) {
	testutil.RequirePOSIX(t)
	testutil.InstallFakePodman(t, "#!/bin/sh\necho fail >&2\nexit 7\n")

	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	w := &Worker{config: Config{
		AgentDir:  agentDir,
		ImageName: "agentfm-test",
	}}

	err := w.buildSandboxImage(context.Background())
	if err == nil {
		t.Fatal("expected error on non-zero podman exit")
	}
	// %w wrap means errors.Is should find the underlying os/exec.ExitError;
	// we don't assert that exact type here (CI variance), just that the error
	// message names the failure.
	if !strings.Contains(err.Error(), "podman build failed") {
		t.Errorf("expected wrapped 'podman build failed', got %v", err)
	}
	// Sanity: the wrapping should preserve the underlying error.
	var unwrapped error
	if errors.Unwrap(err) == nil {
		t.Errorf("expected error to wrap an underlying cause; got %v (unwrap=%v)", err, unwrapped)
	}
}

package worker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/test/testutil"
)

// These tests pin the distinction executePodman must preserve: a task that
// never ran, a task that ran and ended badly, and a task that succeeded are
// three different outcomes. Collapsing them is how a worker ends up telling
// the Boss "Task complete" for compute it never performed.
//
// Every test chdirs into a temp dir first: executePodman anchors its output
// directory on os.Getwd(), so without this the suite would litter
// internal/worker/.agentfm_temp/ in the source tree on every run.

func newSandboxTestWorker(t *testing.T, image string) *Worker {
	t.Helper()
	t.Chdir(t.TempDir())
	return &Worker{config: Config{
		ImageName: image,
		ModelName: "test-model",
		Network:   NetworkHost,
		AgentDir:  t.TempDir(), // no .env inside, so none is passed through
	}}
}

// podman absent from PATH: the process never spawns, so no work happened.
func TestExecutePodman_NotRunWhenPodmanMissing(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	t.Setenv("PATH", t.TempDir()) // empty dir: `podman` is unresolvable

	outputDir, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if !errors.Is(err, ErrSandboxNotRun) {
		t.Fatalf("got error %v, want ErrSandboxNotRun", err)
	}
	if errors.Is(err, ErrSandboxFailed) {
		t.Error("a task that never started must not be reported as ErrSandboxFailed")
	}
	if outputDir == "" {
		t.Error("output directory must still be returned so the caller can clean it up")
	}
}

// A malformed spec must be rejected BEFORE anything is spawned. The fake
// podman writes a marker file; its absence is the proof that no process ran.
func TestExecutePodman_InvalidSpecFailsBeforeSpawn(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "") // empty image → BuildRunArgs rejects the spec

	marker := filepath.Join(t.TempDir(), "podman-was-invoked")
	testutil.InstallFakePodman(t, "#!/bin/sh\ntouch "+marker+"\nexit 0\n")

	_, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if !errors.Is(err, ErrSandboxNotRun) {
		t.Fatalf("got error %v, want ErrSandboxNotRun", err)
	}
	if !errors.Is(err, ErrNoImage) {
		t.Errorf("underlying cause should stay inspectable: got %v, want it to wrap ErrNoImage", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("podman was invoked despite an invalid spec: the spec must be validated before spawning")
	}
}

// The container ran and exited non-zero. Real compute happened, so this is
// ErrSandboxFailed — distinct from never having run, because partial output
// may exist and is still worth collecting.
func TestExecutePodman_FailedOnNonZeroExit(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	testutil.InstallFakePodman(t, "#!/bin/sh\nexit 1\n")

	_, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if !errors.Is(err, ErrSandboxFailed) {
		t.Fatalf("got error %v, want ErrSandboxFailed", err)
	}
	if errors.Is(err, ErrSandboxNotRun) {
		t.Error("a container that ran must not be reported as ErrSandboxNotRun")
	}
}

// A container SIGKILLed by context cancellation (shutdown, task timeout, dead
// Boss stream) did run, so it is a failure rather than a no-run.
//
// Two traps make this test easy to write wrongly, and both were hit before:
//
//   - InstallFakePodman REPLACES $PATH with a directory holding only the fake
//     binary, so a script body of `sleep 30` dies instantly with "sleep: not
//     found" (exit 127). The container then terminates long before the cancel
//     fires, the test passes for the wrong reason, and it keeps passing even
//     if exec.CommandContext is swapped for exec.Command — i.e. it stops
//     testing cancellation entirely. Resolve sleep's absolute path in Go and
//     bake it into the script so the emptied PATH is irrelevant.
//   - The block must be conditional on the `run` subcommand. executePodman's
//     deferred cleanup invokes the same fake binary as `podman rm -f`; an
//     unconditional block hangs that cleanup for its full 10s budget and the
//     test fails on healthy code.
//
// `exec sleep` rather than a shell busy-wait matters: the PID that
// CommandContext kills is then the sleep itself. A `while :; do :; done` loop
// survives as an orphan pinning a core at 100% forever the moment
// cancellation regresses — which is exactly the case this test exists to
// catch.
func TestExecutePodman_FailedOnContextCancel(t *testing.T) {
	testutil.RequirePOSIX(t)

	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary available on this host")
	}

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	testutil.InstallFakePodman(t, "#!/bin/sh\nif [ \"$1\" = \"run\" ]; then exec "+sleepBin+" 60; fi\nexit 0\n")

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)

	// Bound the wait locally so a regression that unwires cancellation fails
	// this one test instead of panicking the whole package on the global
	// timeout. Buffered so the goroutine can never block on a timed-out test.
	done := make(chan error, 1)
	go func() {
		_, runErr := w.executePodman(ctx, "prompt", os.Stdout, os.Stderr)
		done <- runErr
	}()

	select {
	case runErr := <-done:
		if !errors.Is(runErr, ErrSandboxFailed) {
			t.Fatalf("got error %v, want ErrSandboxFailed", runErr)
		}
		if errors.Is(runErr, ErrSandboxNotRun) {
			t.Error("a cancelled container did run; it must not be reported as ErrSandboxNotRun")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancelling the context did not kill the container: executePodman never returned")
	}
}

// A container killed by its memory cgroup exits 137. When a memory ceiling is
// configured, that is attributable and must be reported as such: an operator
// looking at a wall of StatusError cannot tell "my limit is too low" from
// "this agent crashes", and the two call for opposite fixes.
func TestExecutePodman_AttributesOOMKillToTheMemoryLimit(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	w.config.Limits = ResourceLimits{MemoryBytes: 64 << 20}
	testutil.InstallFakePodman(t, "#!/bin/sh\nif [ \"$1\" = \"run\" ]; then exit 137; fi\nexit 0\n")

	_, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if !errors.Is(err, ErrSandboxOOMKilled) {
		t.Fatalf("got error %v, want ErrSandboxOOMKilled", err)
	}
	// The refinement must not break callers that only ask "did it run".
	if !errors.Is(err, ErrSandboxFailed) {
		t.Error("an OOM kill must still satisfy errors.Is(err, ErrSandboxFailed): the container did run")
	}
	if errors.Is(err, ErrSandboxNotRun) {
		t.Error("an OOM-killed container did run; it must not be reported as ErrSandboxNotRun")
	}
}

// Without a configured ceiling, a 137 came from somewhere else — an external
// kill, the host OOM killer, the agent killing itself. Blaming our memory
// limit would be a fabricated diagnosis that sends the operator tuning a
// setting that is not even in effect.
func TestExecutePodman_DoesNotClaimOOMWithoutAMemoryLimit(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1") // no limits configured
	testutil.InstallFakePodman(t, "#!/bin/sh\nif [ \"$1\" = \"run\" ]; then exit 137; fi\nexit 0\n")

	_, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if errors.Is(err, ErrSandboxOOMKilled) {
		t.Error("an OOM kill was attributed to a memory limit that was never set")
	}
	if !errors.Is(err, ErrSandboxFailed) {
		t.Fatalf("got error %v, want ErrSandboxFailed", err)
	}
}

// The limits configured on the worker must actually reach the container. This
// is the test that fails if the wiring from Config to SandboxSpec is dropped.
func TestExecutePodman_AppliesConfiguredLimits(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	w.config.Limits = ResourceLimits{CPUs: 2, MemoryBytes: 128 << 20, PidsLimit: 512}

	argsFile := filepath.Join(t.TempDir(), "args")
	testutil.InstallFakePodman(t, "#!/bin/sh\nif [ \"$1\" = \"run\" ]; then echo \"$@\" > "+argsFile+"; fi\nexit 0\n")

	if _, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr); err != nil {
		t.Fatalf("executePodman: %v", err)
	}

	raw, readErr := os.ReadFile(argsFile)
	if readErr != nil {
		t.Fatalf("fake podman never recorded its arguments: %v", readErr)
	}
	for _, want := range []string{"--cpus 2", "--memory 134217728", "--memory-swap 134217728", "--pids-limit 512"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("configured limit missing from the container's argv: %q not in %q", want, string(raw))
		}
	}
}

// The only path that may report success.
func TestExecutePodman_SucceedsOnCleanExit(t *testing.T) {
	testutil.RequirePOSIX(t)

	w := newSandboxTestWorker(t, "agentfm-test:v1")
	testutil.InstallFakePodman(t, "#!/bin/sh\nexit 0\n")

	outputDir, err := w.executePodman(context.Background(), "prompt", os.Stdout, os.Stderr)

	if err != nil {
		t.Fatalf("clean run returned an error: %v", err)
	}
	if _, statErr := os.Stat(outputDir); statErr != nil {
		t.Errorf("output directory should exist after a clean run: %v", statErr)
	}
}

// RunLocalTest backs `agentfm -mode test`, the command an operator uses to
// validate an agent image. It must exit non-zero when the sandbox never ran
// instead of printing a success banner.
func TestRunLocalTest_PropagatesSandboxFailure(t *testing.T) {
	testutil.RequirePOSIX(t)

	t.Chdir(t.TempDir())

	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "Containerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("write Containerfile: %v", err)
	}

	// Build succeeds, run fails: `podman build ...` exits 0, `podman run ...`
	// exits 1. $1 is the podman subcommand.
	testutil.InstallFakePodman(t, "#!/bin/sh\nif [ \"$1\" = \"run\" ]; then exit 1; fi\nexit 0\n")

	err := RunLocalTest(context.Background(), Config{
		AgentDir:  agentDir,
		ImageName: "agentfm-test:v1",
		ModelName: "test-model",
		Network:   NetworkHost,
	}, "prompt")

	if err == nil {
		t.Fatal("RunLocalTest reported success although the sandbox failed")
	}
	if !errors.Is(err, ErrSandboxFailed) {
		t.Errorf("got error %v, want it to wrap ErrSandboxFailed", err)
	}
}

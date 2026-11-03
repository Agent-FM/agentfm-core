package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pterm/pterm"
)

// Sandbox outcome sentinels. The Boss pays for compute, so "the task never
// executed" and "the task executed and ended badly" must never collapse into
// the same terminal status: the first means no work was done at all, the
// second means real work happened and may have produced partial output.
//
// Before these existed, every failure path returned only an output directory
// and the caller reported metrics.StatusOK — a worker whose container runtime
// was missing or wedged told the Boss "Task complete" for work it had never
// performed. That is the "billed non-execution" failure mode in the threat
// model, reachable by operational accident rather than malice.
var (
	// ErrSandboxNotRun means no container was ever started: the output
	// directory could not be created, the argument vector was invalid, or the
	// podman process failed to spawn. Nothing can have been produced.
	ErrSandboxNotRun = errors.New("sandbox did not run")

	// ErrSandboxFailed means the container started and terminated
	// abnormally — a non-zero exit, or a SIGKILL from context cancellation
	// (shutdown, task timeout, dead Boss stream). Partial artifacts may exist
	// and are still worth collecting.
	ErrSandboxFailed = errors.New("sandbox terminated abnormally")

	// ErrSandboxOOMKilled refines ErrSandboxFailed: the container was killed
	// for exceeding its memory ceiling. Errors carrying it also match
	// ErrSandboxFailed, so callers that only care "did it run" need no change.
	//
	// This is the observable half of roadmap R1: a limit that silently kills
	// tasks is indistinguishable from a flaky agent, and the operator tunes
	// the wrong thing.
	ErrSandboxOOMKilled = errors.New("sandbox exceeded its memory limit")
)

// oomExitCode is what a container killed by the memory cgroup reports:
// 128 + SIGKILL(9).
//
// Cancellation of our own context is cleanly distinguishable: it kills the
// podman client process, which surfaces as "signal: killed" with ExitCode()
// of -1, never 137. A task timeout is therefore never mislabelled as an OOM.
//
// Two other sources of 137 are NOT distinguishable from a cgroup kill by exit
// code alone, so this is a strong heuristic rather than proof:
//   - the host OOM killer under global memory pressure, which may kill this
//     container over a limit it never approached;
//   - an agent that exits 137 of its own accord — and the prompt driving it
//     comes from the Boss.
//
// Both are mislabelled as oom_killed. The consequence is a misleading metric,
// not a security decision, and it only fires when a memory ceiling is actually
// configured. Reading the cgroup's memory.events, or podman inspect's
// .State.OOMKilled before --rm removes the container, would settle it; see
// roadmap R1.
const oomExitCode = 137

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (w *Worker) buildSandboxImage(ctx context.Context) error {
	pterm.Info.Printfln("Checking for Dockerfile in %s...", pterm.Cyan(w.config.AgentDir))
	if _, err := os.Stat(filepath.Join(w.config.AgentDir, "Dockerfile")); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(w.config.AgentDir, "Containerfile")); os.IsNotExist(err) {
			return fmt.Errorf("no Dockerfile or Containerfile found at %s", w.config.AgentDir)
		}
	}

	pterm.Info.Printfln("Building Podman image '%s' (Forcing no-cache)...", pterm.Yellow(w.config.ImageName))

	// CommandContext binds ctx → SIGKILL so a hung `podman build` (registry
	// unreachable, broken Containerfile that wedges RUN) is killable by
	// Ctrl+C. Plain exec.Command would leave the operator with an
	// unkillable worker until SIGKILL of the parent process.
	cmd := exec.CommandContext(ctx, "podman", "build", "--no-cache", "-t", w.config.ImageName, ".")
	cmd.Dir = w.config.AgentDir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman build failed: %w", err)
	}

	pterm.Success.Println("✅ Sandbox Image Built Successfully!")
	fmt.Println()
	return nil
}

// executePodman runs one task in an ephemeral container and returns the host
// directory bind-mounted as the task's output.
//
// The returned error is nil only when the container ran to completion with a
// zero exit status. Callers MUST branch on it: returning the output directory
// alone cannot distinguish "produced nothing" from "never ran". Use errors.Is
// against ErrSandboxNotRun / ErrSandboxFailed to tell the two apart.
//
// The output directory is returned even on error so the caller can still clean
// it up.
func (w *Worker) executePodman(ctx context.Context, prompt string, outStream, errStream io.Writer) (string, error) {
	sessionID := newSessionID()
	containerName := fmt.Sprintf("agentfm-sandbox-%s", sessionID)
	// Cleanup runs on a detached, bounded ctx so a cancelled parent doesn't
	// short-circuit the `podman rm -f` that catches orphaned containers
	// from SIGKILLed `podman run` processes.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "podman", "rm", "-f", containerName).Run()
	}()

	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "." // Fallback just in case
	}

	agentTempBase := filepath.Join(baseDir, ".agentfm_temp")
	absOutputDir := filepath.Join(agentTempBase, fmt.Sprintf("run_%s", sessionID))

	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		fmt.Fprintf(errStream, "❌ Failed to create output dir: %v\n", err)
		return absOutputDir, fmt.Errorf("%w: create output dir: %w", ErrSandboxNotRun, err)
	}

	// Resolve every host-dependent value HERE, then hand a plain data struct
	// to BuildRunArgs. Keeping the probes (GPU present? .env present?) out of
	// the arg builder is what makes the builder unit-testable without a
	// container runtime or a populated filesystem.
	spec := SandboxSpec{
		ContainerName: containerName,
		Image:         w.config.ImageName,
		Prompt:        prompt,
		OutputDir:     absOutputDir,
		ModelName:     w.config.ModelName,
		Network:       w.config.Network,
		Limits:        w.config.Limits,
	}

	if hasGPU, _, _, _ := getGPUStats(); hasGPU {
		spec.GPUDevice = defaultGPUDevice
	}

	envPath := filepath.Join(w.config.AgentDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		spec.EnvFilePath = envPath
	}

	podmanArgs, err := BuildRunArgs(spec)
	if err != nil {
		// Only reachable on a malformed spec (empty image / unknown network
		// mode), i.e. a misconfigured worker. Surface it on the Boss-facing
		// stream and bail before spawning anything, matching how the
		// MkdirAll failure above is reported.
		fmt.Fprintf(errStream, "❌ Failed to build sandbox arguments: %v\n", err)
		return absOutputDir, fmt.Errorf("%w: build run args: %w", ErrSandboxNotRun, err)
	}

	// exec.CommandContext wires ctx cancellation to SIGKILL of the process.
	// When the task ctx is cancelled (shutdown, stream death, or timeout),
	// the Podman sandbox is torn down instantly instead of running to
	// natural completion.
	cmd := exec.CommandContext(ctx, "podman", podmanArgs...)
	cmd.Stdout = outStream
	cmd.Stderr = errStream

	if err := cmd.Start(); err != nil {
		// podman absent from PATH, not executable, container service down.
		// No container exists, so nothing was computed.
		fmt.Fprintf(errStream, "❌ Failed to start task: %v\n", err)
		return absOutputDir, fmt.Errorf("%w: start podman: %w", ErrSandboxNotRun, err)
	}
	if err := cmd.Wait(); err != nil {
		// Non-zero exits and ctx-triggered kills both land here. We surface
		// the error to the caller's stream so the Boss sees it, and report it
		// as a failure so the task is not recorded as a success. The container
		// did run, so the caller still collects whatever it managed to write.
		fmt.Fprintf(errStream, "⚠️  Sandbox exited: %v\n", err)

		// Attribute the kill to the memory ceiling when the evidence supports
		// it, so the operator can tell "my limit is too low" from "this agent
		// is broken". Guarded on a limit actually being set: without one, a
		// 137 came from somewhere else entirely and claiming OOM would be a
		// fabricated diagnosis.
		var exitErr *exec.ExitError
		if spec.Limits.MemoryBytes > 0 && errors.As(err, &exitErr) && exitErr.ExitCode() == oomExitCode {
			fmt.Fprintf(errStream, "🛑 Task exceeded its %s memory limit and was killed.\n",
				FormatSize(spec.Limits.MemoryBytes))
			return absOutputDir, fmt.Errorf("%w: %w: memory ceiling %s: %w",
				ErrSandboxFailed, ErrSandboxOOMKilled, FormatSize(spec.Limits.MemoryBytes), err)
		}

		return absOutputDir, fmt.Errorf("%w: %w", ErrSandboxFailed, err)
	}

	return absOutputDir, nil
}

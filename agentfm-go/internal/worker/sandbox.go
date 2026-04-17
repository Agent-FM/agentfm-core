package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pterm/pterm"
)

func (w *Worker) buildSandboxImage() error {
	pterm.Info.Printfln("Checking for Dockerfile in %s...", pterm.Cyan(w.config.AgentDir))
	if _, err := os.Stat(filepath.Join(w.config.AgentDir, "Dockerfile")); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(w.config.AgentDir, "Containerfile")); os.IsNotExist(err) {
			return fmt.Errorf("no Dockerfile or Containerfile found at %s", w.config.AgentDir)
		}
	}

	pterm.Info.Printfln("Building Podman image '%s' (Forcing no-cache)...", pterm.Yellow(w.config.ImageName))

	cmd := exec.Command("podman", "build", "--no-cache", "-t", w.config.ImageName, ".")
	cmd.Dir = w.config.AgentDir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman build failed: %w", err)
	}

	pterm.Success.Println("✅ Sandbox Image Built Successfully!\n")
	return nil
}

func (w *Worker) executePodman(ctx context.Context, prompt string, outStream, errStream io.Writer) string {
	sessionID := time.Now().UnixNano()
	containerName := fmt.Sprintf("agentfm-sandbox-%d", sessionID)
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
	absOutputDir := filepath.Join(agentTempBase, fmt.Sprintf("run_%d", sessionID))

	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		fmt.Fprintf(errStream, "❌ Failed to create output dir: %v\n", err)
		return absOutputDir
	}

	podmanArgs := []string{"run", "--rm", "--name", containerName, "--network", "host"}
	hasGPU, _, _, _ := getGPUStats()
	if hasGPU {
		podmanArgs = append(podmanArgs, "--device", "nvidia.com/gpu=all")
	}

	podmanArgs = append(podmanArgs, "-v", fmt.Sprintf("%s:/tmp/output:z", absOutputDir))

	envPath := filepath.Join(w.config.AgentDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		podmanArgs = append(podmanArgs, "--env-file", envPath)
	}

	podmanArgs = append(podmanArgs, "-e", fmt.Sprintf("AGENTFM_MODEL=%s", w.config.ModelName))
	podmanArgs = append(podmanArgs, w.config.ImageName, prompt)

	// exec.CommandContext wires ctx cancellation to SIGKILL of the process.
	// When the task ctx is cancelled (shutdown, stream death, or timeout),
	// the Podman sandbox is torn down instantly instead of running to
	// natural completion.
	cmd := exec.CommandContext(ctx, "podman", podmanArgs...)
	cmd.Stdout = outStream
	cmd.Stderr = errStream

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(errStream, "❌ Failed to start task: %v\n", err)
		return absOutputDir
	}
	if err := cmd.Wait(); err != nil {
		// Non-zero exits and ctx-triggered kills both land here. We surface
		// the error to the caller's stream so the Boss sees it.
		fmt.Fprintf(errStream, "⚠️  Sandbox exited: %v\n", err)
	}

	return absOutputDir
}

package worker

import (
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

func (w *Worker) executePodman(prompt string, outStream, errStream io.Writer) string {
	sessionID := time.Now().UnixNano()
	containerName := fmt.Sprintf("agentfm-sandbox-%d", sessionID)
	defer exec.Command("podman", "rm", "-f", containerName).Run()

	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "." // Fallback just in case
	}

	agentTempBase := filepath.Join(baseDir, ".agentfm_temp")
	absOutputDir := filepath.Join(agentTempBase, fmt.Sprintf("run_%d", sessionID))

	os.MkdirAll(absOutputDir, 0777)
	os.Chmod(absOutputDir, 0777)

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

	cmd := exec.Command("podman", podmanArgs...)
	cmd.Stdout = outStream
	cmd.Stderr = errStream

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(errStream, "❌ Failed to start task: %v\n", err)
	} else {
		cmd.Wait()
	}

	return absOutputDir
}

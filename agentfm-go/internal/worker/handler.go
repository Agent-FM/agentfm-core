package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"agentfm/internal/network"
	"agentfm/internal/utils"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/pterm/pterm"
)

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err != nil
}

func (w *Worker) handleTaskStream(s netcore.Stream) {
	defer s.Close()
	fmt.Println()
	pterm.Info.Println("Incoming P2P task tunnel established...")

	bossID := s.Conn().RemotePeer()

	w.mu.Lock()
	if w.currentTasks >= w.config.MaxConcurrentTasks {
		w.mu.Unlock()
		pterm.Error.Printfln("Rejected task: Worker is at max capacity (%d/%d).", w.currentTasks, w.config.MaxConcurrentTasks)
		s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker is at max capacity (%d/%d). Try another worker.\n", w.currentTasks, w.config.MaxConcurrentTasks)))
		return
	}

	w.currentTasks++
	cpuLoad := w.currentCPU
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.currentTasks--
		w.mu.Unlock()
	}()

	if cpuLoad >= w.config.MaxCPU { // Dynamic Threshold
		pterm.Error.Printfln("Rejected task: Worker CPU is overloaded at %.1f%%.", cpuLoad)
		s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker is under heavy load (CPU %.1f%%). Try again later.\n", cpuLoad)))
		return
	}

	hasGPU, _, _, gpuPct := getGPUStats()
	if hasGPU && gpuPct > w.config.MaxGPU { // Dynamic Threshold
		pterm.Error.Printfln("Rejected task: Worker GPU VRAM is busy (%.1f%% used).", gpuPct)
		s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker GPU is busy (%.1f%% VRAM used). Try another worker.\n", gpuPct)))
		return
	}

	var payload struct {
		Version string `json:"version"`
		Task    string `json:"task"`
		Data    string `json:"data"`
		TaskID  string `json:"task_id"`
	}

	limitedReader := io.LimitReader(s, 1*1024*1024)

	if err := json.NewDecoder(limitedReader).Decode(&payload); err != nil {
		pterm.Error.Println("Failed to decode incoming task payload (or payload exceeded 1MB limit).")
		return
	}

	if payload.Version != version.AppVersion {
		s.Write([]byte(fmt.Sprintf("❌ ERROR: Version mismatch! Worker is running v%s.\n", version.AppVersion)))
		return
	}

	pterm.Info.Printfln("Executing task %s in Podman sandbox...", payload.TaskID)

	outputDir := w.executePodman(payload.Data, s, s)
	defer os.RemoveAll(outputDir)

	if !isDirEmpty(outputDir) {
		s.Write([]byte("\n[AGENTFM: FILES_INCOMING]\n"))
		pterm.Info.Println("Artifacts detected. Preparing zip...")

		zipPath := outputDir + "_payload.zip"
		defer os.Remove(zipPath)

		if err := utils.ZipDirectory(outputDir, zipPath); err == nil {
			pterm.Info.Println("Routing artifacts to Boss over secure channel...")
			if sendErr := network.SendArtifacts(context.Background(), w.node.Host, bossID, zipPath, payload.TaskID); sendErr != nil {
				pterm.Error.Printfln("Failed to route artifacts: %v", sendErr)
			}
		} else {
			pterm.Error.Printfln("Failed to zip artifacts: %v", err)
		}
	} else {
		s.Write([]byte("\n[AGENTFM: NO_FILES]\n"))
		pterm.Success.Println("No artifacts generated. Task complete.")
	}
}

func (w *Worker) handleFeedbackStream(s netcore.Stream) {
	defer s.Close()
	var payload struct {
		Task      string `json:"task"`
		Feedback  string `json:"feedback"`
		Timestamp string `json:"timestamp"`
	}

	limitedReader := io.LimitReader(s, 1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(&payload); err != nil {
		return
	}

	fmt.Println()
	pterm.DefaultBox.WithTitle(pterm.LightYellow("💌 NEW FEEDBACK RECEIVED")).Printfln("Agent: %s\nTask: %s\nFeedback: %s", pterm.Magenta(w.config.AgentName), pterm.Cyan(payload.Task), pterm.White(payload.Feedback))

	logPath := filepath.Join(w.config.AgentDir, "feedback.log")
	if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] Task: %s | Feedback: %s\n", payload.Timestamp, payload.Task, payload.Feedback))
	}
}

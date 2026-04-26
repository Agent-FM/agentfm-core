package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/types"
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

func (w *Worker) handleTaskStream(rootCtx context.Context, s netcore.Stream) {
	// Per-task observability — declared first so the deferred closure runs
	// LAST (Go runs defers in LIFO), capturing the full lifetime including
	// the stream Close/Reset.
	started := time.Now()
	status := metrics.StatusError
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(started).Seconds())
		metrics.TasksTotal.WithLabelValues(status).Inc()
	}()

	// Default to Reset on any early/error exit. A caller that reaches the
	// normal end of the task flips this to a graceful Close.
	reset := true
	defer func() {
		if reset {
			_ = s.Reset()
		} else {
			_ = s.Close()
		}
	}()

	// Short deadline for the incoming task JSON. Extended below once the
	// payload is accepted and we start streaming stdout back. If arming
	// the deadline fails the stream is already unhealthy, so we surface
	// it and let the Reset defer clean up.
	if err := s.SetDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonDeadline).Inc()
		slog.Error("arm task stream deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "task"))
		return
	}

	fmt.Println()
	pterm.Info.Println("Incoming P2P task tunnel established...")

	bossID := s.Conn().RemotePeer()

	w.mu.Lock()
	if w.currentTasks >= w.config.MaxConcurrentTasks {
		w.mu.Unlock()
		status = metrics.StatusRejected
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonCapacityRejected).Inc()
		pterm.Error.Printfln("Rejected task: Worker is at max capacity (%d/%d).", w.currentTasks, w.config.MaxConcurrentTasks)
		_, _ = s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker is at max capacity (%d/%d). Try another worker.\n", w.currentTasks, w.config.MaxConcurrentTasks)))
		// App-level rejection delivered — close gracefully so the peer sees the message.
		reset = false
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
		status = metrics.StatusRejected
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonCapacityRejected).Inc()
		pterm.Error.Printfln("Rejected task: Worker CPU is overloaded at %.1f%%.", cpuLoad)
		_, _ = s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker is under heavy load (CPU %.1f%%). Try again later.\n", cpuLoad)))
		reset = false
		return
	}

	hasGPU, _, _, gpuPct := getGPUStats()
	if hasGPU && gpuPct > w.config.MaxGPU { // Dynamic Threshold
		status = metrics.StatusRejected
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonCapacityRejected).Inc()
		pterm.Error.Printfln("Rejected task: Worker GPU VRAM is busy (%.1f%% used).", gpuPct)
		_, _ = s.Write([]byte(fmt.Sprintf("❌ ERROR: Worker GPU is busy (%.1f%% VRAM used). Try another worker.\n", gpuPct)))
		reset = false
		return
	}

	var payload types.TaskPayload

	limitedReader := io.LimitReader(s, 1*1024*1024)

	if err := json.NewDecoder(limitedReader).Decode(&payload); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonDecode).Inc()
		slog.Error("decode task payload (or >1MiB)", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "task"))
		return
	}

	if payload.Version != version.AppVersion {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonVersionMismatch).Inc()
		_, _ = s.Write([]byte(fmt.Sprintf("❌ ERROR: Version mismatch! Worker is running v%s.\n", version.AppVersion)))
		reset = false
		return
	}

	// Payload accepted. Extend the deadline to cover long running stdout
	// streaming. If the extension fails, abort now so we don't run a
	// Podman container whose output channel is already doomed.
	if err := s.SetDeadline(time.Now().Add(network.TaskExecutionTimeout)); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonDeadline).Inc()
		slog.Error("extend task stream deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, payload.TaskID), slog.String(obs.FieldProtocol, "task"))
		return
	}

	// Task-scoped ctx: cancels on worker shutdown (rootCtx), on
	// TaskExecutionTimeout, and on remote conn death (watcher below).
	// Passed to executePodman so exec.CommandContext SIGKILLs the
	// container the instant the tunnel dies.
	taskCtx, cancelTask := context.WithTimeout(rootCtx, network.TaskExecutionTimeout)
	defer cancelTask()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				if s.Conn().IsClosed() {
					pterm.Warning.Println("Remote Boss connection died; cancelling sandbox...")
					cancelTask()
					return
				}
			}
		}
	}()

	pterm.Info.Printfln("Executing task %s in Podman sandbox...", payload.TaskID)

	outputDir := w.executePodman(taskCtx, payload.Data, s, s)
	defer os.RemoveAll(outputDir)

	if !isDirEmpty(outputDir) {
		_, _ = s.Write([]byte("\n[AGENTFM: FILES_INCOMING]\n"))
		pterm.Info.Println("Artifacts detected. Preparing zip...")

		zipPath := outputDir + "_payload.zip"
		defer os.Remove(zipPath)

		if err := utils.ZipDirectory(outputDir, zipPath); err == nil {
			pterm.Info.Println("Routing artifacts to Boss over secure channel...")
			artifactCtx, cancelArtifact := context.WithTimeout(rootCtx, network.ArtifactStreamTimeout)
			if sendErr := network.SendArtifacts(artifactCtx, w.node.Host, bossID, zipPath, payload.TaskID); sendErr != nil {
				slog.Error("route artifacts", slog.Any(obs.FieldErr, sendErr), slog.String(obs.FieldTaskID, payload.TaskID), slog.String(obs.FieldProtocol, "artifacts"))
			}
			cancelArtifact()
		} else {
			slog.Error("zip artifacts", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, payload.TaskID))
		}
	} else {
		_, _ = s.Write([]byte("\n[AGENTFM: NO_FILES]\n"))
		pterm.Success.Println("No artifacts generated. Task complete.")
	}

	reset = false
	status = metrics.StatusOK
}

func (w *Worker) handleFeedbackStream(ctx context.Context, s netcore.Stream) {
	reset := true
	defer func() {
		if reset {
			_ = s.Reset()
		} else {
			_ = s.Close()
		}
	}()

	if err := s.SetDeadline(time.Now().Add(network.FeedbackStreamTimeout)); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolFeedback, metrics.ReasonDeadline).Inc()
		slog.Error("arm feedback stream deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "feedback"))
		return
	}

	if err := ctx.Err(); err != nil {
		// Worker shutting down — refuse the write rather than touch disk.
		return
	}

	var payload struct {
		Task      string `json:"task"`
		Feedback  string `json:"feedback"`
		Timestamp string `json:"timestamp"`
	}

	limitedReader := io.LimitReader(s, 1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(&payload); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolFeedback, metrics.ReasonDecode).Inc()
		slog.Error("decode feedback payload", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "feedback"))
		return
	}

	fmt.Println()
	pterm.DefaultBox.WithTitle(pterm.LightYellow("💌 NEW FEEDBACK RECEIVED")).Printfln("Agent: %s\nTask: %s\nFeedback: %s", pterm.Magenta(w.config.AgentName), pterm.Cyan(payload.Task), pterm.White(payload.Feedback))

	// Feedback may contain user-private prose. 0600 instead of 0644.
	logPath := filepath.Join(w.config.AgentDir, "feedback.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("open feedback log", slog.Any(obs.FieldErr, err), slog.String("path", logPath))
		return
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "[%s] Task: %s | Feedback: %s\n", payload.Timestamp, payload.Task, payload.Feedback); err != nil {
		slog.Error("write feedback log", slog.Any(obs.FieldErr, err), slog.String("path", logPath))
		return
	}

	reset = false
}

package boss

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/types"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

// handleGetWorkers serves the /api/workers listing. It also doubles as a
// self cleaning pass: any worker whose peer we are no longer connected to
// is pruned from activeWorkers (and its lastSeen entry) while serving the
// request, so the radar never drifts.
func (b *Boss) handleGetWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b.mu.Lock()
	agents := make([]apiWorker, 0, len(b.activeWorkers))

	for peerIDStr, profile := range b.activeWorkers {

		pID, err := peer.Decode(peerIDStr)
		if err == nil {
			if b.node.Host.Network().Connectedness(pID) != netcore.Connected {
				// Mirror the eviction logic in ui.go by clearing both
				// maps. Leaving lastSeen populated for a peer we just
				// removed from activeWorkers leaks a string key forever
				// and confuses any future tick that reads lastSeen.
				delete(b.activeWorkers, peerIDStr)
				delete(b.lastSeen, peerIDStr)
				continue
			}
		}

		agents = append(agents, profileToAPIWorker(profile))
	}
	online := len(b.activeWorkers)
	b.mu.Unlock()
	metrics.WorkersOnline.Set(float64(online))

	response := map[string]interface{}{
		"success": true,
		"agents":  agents,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("encode /api/workers response", slog.Any(obs.FieldErr, err))
	}
}

func profileToAPIWorker(p types.WorkerProfile) apiWorker {
	hardwareStr := fmt.Sprintf("%s (CPU: %d Cores)", p.Model, p.CPUCores)
	if p.HasGPU {
		hardwareStr = fmt.Sprintf("%s (GPU VRAM: %.1f/%.1f GB)", p.Model, p.GPUUsedGB, p.GPUTotalGB)
	}
	return apiWorker{
		PeerID:       p.PeerID,
		Author:       p.Author,
		Name:         p.AgentName,
		Status:       p.Status,
		Hardware:     hardwareStr,
		Description:  p.AgentDesc,
		CPUUsagePct:  p.CPUUsagePct,
		RAMFreeGB:    p.RAMFreeGB,
		CurrentTasks: p.CurrentTasks,
		MaxTasks:     p.MaxTasks,
		HasGPU:       p.HasGPU,
		GPUUsedGB:    p.GPUUsedGB,
		GPUTotalGB:   p.GPUTotalGB,
		GPUUsagePct:  p.GPUUsagePct,
	}
}

// handleExecuteTask implements POST /api/execute. It dials the worker,
// streams the task prompt, and proxies the worker's stdout back to the
// HTTP client in real time so the SDK can render progress as it arrives.
func (b *Boss) handleExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Per-task observability: histogram is observed unconditionally; the
	// counter is incremented exactly once with the terminal status. The
	// status variable is mutated as the handler progresses; whatever value
	// it holds at function return is what's reported.
	started := time.Now()
	status := metrics.StatusError
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(started).Seconds())
		metrics.TasksTotal.WithLabelValues(status).Inc()
	}()

	var req ExecuteRequest
	limitedReader := io.LimitReader(r.Body, 1*1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		req.TaskID = newCompletionID("task_")
	}

	b.mu.RLock()
	_, exists := b.activeWorkers[req.WorkerID]
	b.mu.RUnlock()

	if !exists {
		status = metrics.StatusRejected
		http.Error(w, "Worker not found or offline", http.StatusNotFound)
		return
	}

	peerID, err := peer.Decode(req.WorkerID)
	if err != nil {
		http.Error(w, "Invalid Worker ID format", http.StatusBadRequest)
		return
	}

	pterm.Info.Printfln("📡 API Gateway routing task %s to Worker %s...", req.TaskID[:8], pterm.Cyan(peerID.String()[:8]))

	// Tie the dial to the inbound HTTP request's context so a client
	// hanging up aborts the libp2p dial instead of waiting out the full
	// StreamDialTimeout.
	s := b.dialOmni(r.Context(), peerID)
	if s == nil {
		http.Error(w, "Failed to connect to worker via DHT or Relay", http.StatusInternalServerError)
		return
	}

	// Default to Reset on any early exit so a half written frame cannot
	// leave the worker waiting on a dead stream. The success path flips
	// this at the end to trigger a graceful Close.
	streamSuccess := false
	defer func() {
		if streamSuccess {
			_ = s.Close()
		} else {
			_ = s.Reset()
		}
	}()

	if err := s.SetWriteDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
		http.Error(w, "Failed to set write deadline", http.StatusInternalServerError)
		return
	}

	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    req.Prompt,
		TaskID:  req.TaskID,
	}
	if err := json.NewEncoder(s).Encode(&payload); err != nil {
		http.Error(w, "Failed to send prompt", http.StatusInternalServerError)
		return
	}
	if err := s.CloseWrite(); err != nil {
		http.Error(w, "Failed to half-close task stream", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, flusherSupported := w.(http.Flusher)
	buf := make([]byte, 1024) // Read in small 1KB chunks

	for {
		// Idle deadline. Each read gets up to TaskExecutionTimeout. A task
		// that never produces output within that window is treated as ghosted.
		if err := s.SetReadDeadline(time.Now().Add(network.TaskExecutionTimeout)); err != nil {
			slog.Error("refresh read deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, req.TaskID))
			return
		}
		n, err := s.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				slog.Warn("HTTP client disconnected", slog.Any(obs.FieldErr, werr), slog.String(obs.FieldTaskID, req.TaskID))
				return
			}
			if flusherSupported {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				slog.Error("worker stream", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, req.TaskID), slog.String(obs.FieldProtocol, "task"))
				return
			}
			break
		}
	}

	streamSuccess = true
	status = metrics.StatusOK
	pterm.Success.Println("✅ API Task Complete. Text streamed to client.")
}

package boss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

type ExecuteRequest struct {
	WorkerID string `json:"worker_id"`
	Prompt   string `json:"prompt"`
	TaskID   string `json:"task_id"`
}

type AsyncExecuteRequest struct {
	WorkerID   string `json:"worker_id"`
	Prompt     string `json:"prompt"`
	WebhookURL string `json:"webhook_url"`
}

// corsMiddleware wraps standard handlers to easily attach headers
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func (b *Boss) StartAPIServer(port string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)
	go b.listenTelemetry(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/workers", corsMiddleware(b.handleGetWorkers))
	mux.HandleFunc("/api/execute", corsMiddleware(b.handleExecuteTask))
	mux.HandleFunc("/api/execute/async", corsMiddleware(b.handleAsyncExecuteTask))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Run server in a background goroutine
	go func() {
		pterm.Success.Printfln("🚀 AgentFM Local API Gateway listening on http://127.0.0.1:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			pterm.Fatal.Printfln("❌ API Server failed: %v", err)
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	pterm.Warning.Println("\nShutting down API Gateway gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		pterm.Error.Printfln("Server forced to shutdown: %v", err)
	}

	pterm.Success.Println("API Gateway offline.")
	os.Exit(0)
}

func (b *Boss) handleGetWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	type APIWorker struct {
		PeerID       string  `json:"peer_id"`
		Author       string  `json:"author"`
		Name         string  `json:"name"`
		Status       string  `json:"status"`
		Hardware     string  `json:"hardware"`
		Description  string  `json:"description"`
		CPUUsagePct  float64 `json:"cpu_usage_pct"`
		RAMFreeGB    float64 `json:"ram_free_gb"`
		CurrentTasks int     `json:"current_tasks"`
		MaxTasks     int     `json:"max_tasks"`
		HasGPU       bool    `json:"has_gpu"`
		GPUUsedGB    float64 `json:"gpu_used_gb"`
		GPUTotalGB   float64 `json:"gpu_total_gb"`
		GPUUsagePct  float64 `json:"gpu_usage_pct"`
	}

	agents := make([]APIWorker, 0)

	for peerIDStr, profile := range b.activeWorkers {

		pID, err := peer.Decode(peerIDStr)
		if err == nil {
			if b.node.Host.Network().Connectedness(pID) != netcore.Connected {
				delete(b.activeWorkers, peerIDStr)
				continue
			}
		}

		hardwareStr := fmt.Sprintf("%s (CPU: %d Cores)", profile.Model, profile.CPUCores)
		if profile.HasGPU {
			hardwareStr = fmt.Sprintf("%s (GPU VRAM: %.1f/%.1f GB)", profile.Model, profile.GPUUsedGB, profile.GPUTotalGB)
		}

		agents = append(agents, APIWorker{
			PeerID:       profile.PeerID,
			Author:       profile.Author,
			Name:         profile.AgentName,
			Status:       profile.Status,
			Hardware:     hardwareStr,
			Description:  profile.AgentDesc,
			CPUUsagePct:  profile.CPUUsagePct,
			RAMFreeGB:    profile.RAMFreeGB,
			CurrentTasks: profile.CurrentTasks,
			MaxTasks:     profile.MaxTasks,
			HasGPU:       profile.HasGPU,
			GPUUsedGB:    profile.GPUUsedGB,
			GPUTotalGB:   profile.GPUTotalGB,
			GPUUsagePct:  profile.GPUUsagePct,
		})
	}

	response := map[string]interface{}{
		"success": true,
		"agents":  agents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *Boss) handleExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecuteRequest
	limitedReader := io.LimitReader(r.Body, 1*1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		req.TaskID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}

	b.mu.Lock()
	_, exists := b.activeWorkers[req.WorkerID]
	b.mu.Unlock()

	if !exists {
		http.Error(w, "Worker not found or offline", http.StatusNotFound)
		return
	}

	peerID, err := peer.Decode(req.WorkerID)
	if err != nil {
		http.Error(w, "Invalid Worker ID format", http.StatusBadRequest)
		return
	}

	pterm.Info.Printfln("📡 API Gateway routing task %s to Worker %s...", req.TaskID[:8], pterm.Cyan(peerID.String()[:8]))

	s := b.dialOmni(peerID)
	if s == nil {
		http.Error(w, "Failed to connect to worker via DHT or Relay", http.StatusInternalServerError)
		return
	}

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

	taskJSON := fmt.Sprintf(`{"version": "%s", "task": "agent_task", "data": "%s", "task_id": "%s"}`, version.AppVersion, req.Prompt, req.TaskID)
	if _, err := s.Write([]byte(taskJSON)); err != nil {
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
		// Idle deadline: each read gets up to TaskExecutionTimeout. A task that
		// never produces output within that window is treated as ghosted.
		if err := s.SetReadDeadline(time.Now().Add(network.TaskExecutionTimeout)); err != nil {
			pterm.Error.Printfln("Failed to refresh read deadline: %v", err)
			return
		}
		n, err := s.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				pterm.Error.Printfln("HTTP client disconnected: %v", werr)
				return
			}
			if flusherSupported {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				pterm.Error.Printfln("Stream error: %v", err)
				return
			}
			break
		}
	}

	streamSuccess = true
	pterm.Success.Println("✅ API Task Complete. Text streamed to client.")
}

func (b *Boss) handleAsyncExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AsyncExecuteRequest
	limitedReader := io.LimitReader(r.Body, 1*1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	_, exists := b.activeWorkers[req.WorkerID]
	b.mu.Unlock()

	if !exists {
		http.Error(w, "Worker not found or offline", http.StatusNotFound)
		return
	}

	peerID, err := peer.Decode(req.WorkerID)
	if err != nil {
		http.Error(w, "Invalid Worker ID format", http.StatusBadRequest)
		return
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"task_id": taskID,
		"status":  "queued",
		"message": "Task dispatched to P2P mesh.",
	})

	go func() {
		s := b.dialOmni(peerID)
		if s == nil {
			return
		}

		streamSuccess := false
		defer func() {
			if streamSuccess {
				_ = s.Close()
			} else {
				_ = s.Reset()
			}
		}()

		if err := s.SetWriteDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
			pterm.Error.Printfln("Async task: failed to set write deadline: %v", err)
			return
		}

		taskJSON := fmt.Sprintf(`{"version": "%s", "task": "agent_task", "data": "%s", "task_id": "%s"}`, version.AppVersion, req.Prompt, taskID)
		if _, err := s.Write([]byte(taskJSON)); err != nil {
			pterm.Error.Printfln("Async task: failed to send prompt: %v", err)
			return
		}
		if err := s.CloseWrite(); err != nil {
			pterm.Error.Printfln("Async task: failed to half-close: %v", err)
			return
		}

		deadman := &timeoutReader{stream: s, timeout: network.TaskExecutionTimeout}
		var outputBuffer bytes.Buffer
		if _, err := io.Copy(&outputBuffer, deadman); err != nil {
			pterm.Error.Printfln("Async task: stream read failed: %v", err)
			return
		}

		streamSuccess = true

		time.Sleep(2 * time.Second)

		if req.WebhookURL != "" {
			webhookPayload := map[string]string{
				"task_id":   taskID,
				"worker_id": req.WorkerID,
				"status":    "completed",
			}
			jsonData, _ := json.Marshal(webhookPayload)
			resp, err := http.Post(req.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
}

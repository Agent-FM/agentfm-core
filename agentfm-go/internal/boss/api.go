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
	"sync"
	"syscall"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

const webhookTimeout = 30 * time.Second

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

// apiWorker is the response DTO for GET /api/workers. It flattens a
// WorkerProfile into the fields the SDK/UI consumes and renames a few
// (PeerID vs peer_id is identical; AgentName becomes name, etc.).
type apiWorker struct {
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

func (b *Boss) StartAPIServer(port string) error {
	// Root ctx cancels on SIGINT/SIGTERM so every downstream goroutine
	// (telemetry listener, async task workers, webhook POSTs) observes
	// the same shutdown signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Tracks in-flight async task goroutines spawned by /api/execute/async.
	// http.Server.Shutdown only drains HTTP handlers; these goroutines
	// outlive the handler return, so we drain them explicitly below.
	var asyncWG sync.WaitGroup

	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)
	go b.listenTelemetry(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/workers", corsMiddleware(b.handleGetWorkers))
	mux.HandleFunc("/api/execute", corsMiddleware(b.handleExecuteTask))
	mux.HandleFunc("/api/execute/async", corsMiddleware(b.asyncExecuteHandler(ctx, &asyncWG)))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
		// Defensive server timeouts so slow-loris clients cannot exhaust
		// handler goroutines.
		// - ReadHeaderTimeout specifically guards against slow header writes
		//   (the classic slow-loris vector).
		// - ReadTimeout bounds the whole request body (small JSON).
		// - WriteTimeout must be larger than TaskExecutionTimeout because
		//   /api/execute streams worker stdout for the full task window.
		// - IdleTimeout reaps dormant keep-alive connections.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      network.TaskExecutionTimeout + 2*time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	// Run the server in a background goroutine and route its terminal
	// error through a channel instead of calling pterm.Fatal (which
	// would os.Exit(1) from inside a goroutine, skipping every defer
	// in StartAPIServer). A caller-observable return value lets
	// main.go decide the exit code.
	serverErrCh := make(chan error, 1)
	go func() {
		pterm.Success.Printfln("🚀 AgentFM Local API Gateway listening on http://127.0.0.1:%s", port)
		serverErrCh <- srv.ListenAndServe()
	}()

	var listenErr error
	select {
	case <-ctx.Done():
		pterm.Warning.Println("\nShutting down API Gateway gracefully...")
	case err := <-serverErrCh:
		// Server exited before a shutdown signal — typically a bind
		// failure at startup. Fall through to the drain path so
		// telemetry/async goroutines still get cleaned up, then
		// propagate the error to main.
		if err != nil && err != http.ErrServerClosed {
			pterm.Error.Printfln("API Server failed: %v", err)
			listenErr = err
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		pterm.Error.Printfln("Server forced to shutdown: %v", err)
	}

	// Wait for async task goroutines to finish, bounded so a hung webhook
	// cannot block shutdown forever.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer drainCancel()
	drained := make(chan struct{})
	go func() {
		asyncWG.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		pterm.Success.Println("Async tasks drained.")
	case <-drainCtx.Done():
		pterm.Warning.Println("Drain deadline hit — some async tasks still in flight.")
	}

	pterm.Success.Println("API Gateway offline.")
	return listenErr
}

func (b *Boss) handleGetWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	agents := make([]apiWorker, 0, len(b.activeWorkers))

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

		agents = append(agents, apiWorker{
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

	b.mu.RLock()
	_, exists := b.activeWorkers[req.WorkerID]
	b.mu.RUnlock()

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

// asyncExecuteHandler returns the /api/execute/async handler wired to the
// server-lifetime rootCtx and an inflight WaitGroup. The handler spawns a
// background goroutine after responding 202; that goroutine inherits rootCtx
// (so SIGINT cancels it) and calls wg.Done() on exit so StartAPIServer can
// wait for an orderly drain.
func (b *Boss) asyncExecuteHandler(rootCtx context.Context, wg *sync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		b.mu.RLock()
		_, exists := b.activeWorkers[req.WorkerID]
		b.mu.RUnlock()

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
		if err := json.NewEncoder(w).Encode(map[string]string{
			"task_id": taskID,
			"status":  "queued",
			"message": "Task dispatched to P2P mesh.",
		}); err != nil {
			pterm.Error.Printfln("Async task: failed to write ack: %v", err)
			return
		}

		// Budget the whole background job (stream + webhook) by the task
		// execution timeout so a ghosted worker can't hold a goroutine
		// past server shutdown.
		taskCtx, cancelTask := context.WithTimeout(rootCtx, network.TaskExecutionTimeout)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancelTask()

			b.runAsyncTask(taskCtx, peerID, taskID, req)
		}()
	}
}

func (b *Boss) runAsyncTask(ctx context.Context, peerID peer.ID, taskID string, req AsyncExecuteRequest) {
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

	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    req.Prompt,
		TaskID:  taskID,
	}
	if err := json.NewEncoder(s).Encode(&payload); err != nil {
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

	// Brief grace period so the out-of-band artifact stream (handled by
	// HandleArtifactStream on a separate libp2p stream) lands on disk
	// before we notify the webhook. Aborted early if the server is
	// shutting down.
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}

	if req.WebhookURL == "" {
		return
	}

	webhookPayload := map[string]string{
		"task_id":   taskID,
		"worker_id": req.WorkerID,
		"status":    "completed",
	}
	jsonData, err := json.Marshal(webhookPayload)
	if err != nil {
		pterm.Error.Printfln("Async task: failed to marshal webhook payload: %v", err)
		return
	}

	// Bounded webhook POST. The default http.Client has NO timeout, so a
	// hostile or slow webhook URL would otherwise stall this goroutine —
	// and therefore the server drain — indefinitely.
	webhookCtx, webhookCancel := context.WithTimeout(ctx, webhookTimeout)
	defer webhookCancel()

	httpReq, err := http.NewRequestWithContext(webhookCtx, http.MethodPost, req.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		pterm.Error.Printfln("Async task: failed to build webhook request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: webhookTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		pterm.Error.Printfln("Async task: webhook delivery failed: %v", err)
		return
	}
	_ = resp.Body.Close()
}

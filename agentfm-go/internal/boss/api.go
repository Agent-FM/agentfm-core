package boss

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agentfm/internal/network"

	"github.com/pterm/pterm"
)

// ExecuteRequest is the request body accepted by POST /api/execute. A
// missing task_id is accepted and filled in by the handler so SDK clients
// that only care about the streamed response don't have to synthesise one.
type ExecuteRequest struct {
	WorkerID string `json:"worker_id"`
	Prompt   string `json:"prompt"`
	TaskID   string `json:"task_id"`
}

// AsyncExecuteRequest is the request body for POST /api/execute/async.
// WebhookURL is optional; when empty the Boss finishes the task quietly
// and writes artifacts to disk without notifying anyone.
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
	mux.HandleFunc("/v1/models", corsMiddleware(b.handleModels))
	mux.HandleFunc("/v1/chat/completions", corsMiddleware(b.handleChatCompletions))
	mux.HandleFunc("/v1/completions", corsMiddleware(b.handleCompletions))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
		// Defensive server timeouts so slow-loris clients cannot exhaust
		// handler goroutines.
		// ReadHeaderTimeout specifically guards against slow header writes
		// (the classic slow-loris vector).
		// ReadTimeout bounds the whole request body (small JSON).
		// WriteTimeout must be larger than TaskExecutionTimeout because
		// /api/execute streams worker stdout for the full task window.
		// IdleTimeout reaps dormant keep-alive connections.
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
		// Server exited before a shutdown signal, typically a bind
		// failure at startup. We still fall through to the drain path
		// so telemetry/async goroutines get cleaned up, then propagate
		// the error to main.
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
		pterm.Warning.Println("Drain deadline hit, some async tasks still in flight.")
	}

	pterm.Success.Println("API Gateway offline.")
	return listenErr
}

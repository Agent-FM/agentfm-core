package boss

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agentfm/internal/metrics"
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

func (b *Boss) StartAPIServer(bind, port string) error {
	// Bearer-auth config FIRST so the startup-refusal guard fires before
	// we open any listening socket. A failing-fast public deploy never
	// gets a chance to serve a single unauthenticated request.
	auth, err := newAuthConfig()
	if err != nil {
		return fmt.Errorf("auth config: %w", err)
	}
	if err := enforceStartupAuthGuard(bind, auth.tokens); err != nil {
		return err
	}

	// Root ctx cancels on SIGINT/SIGTERM so every downstream goroutine
	// (telemetry listener, async task workers, webhook POSTs) observes
	// the same shutdown signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Tracks in-flight async task goroutines spawned by /api/execute/async
	// AND the long-lived listenTelemetry goroutine. http.Server.Shutdown
	// only drains HTTP handlers; these goroutines outlive the handler
	// return, so we drain them explicitly below before letting the
	// process exit.
	var bgWG sync.WaitGroup

	b.node.Host.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)
	bgWG.Add(1)
	go func() {
		defer bgWG.Done()
		b.listenTelemetry(ctx)
	}()

	// Bearer-auth middleware. When AGENTFM_API_KEYS is unset the middleware
	// is a transparent pass-through (back-compat solo-dev mode); when keys
	// are configured, /api/* and /v1/* require Authorization: Bearer <tok>.
	// /metrics and /health stay open for Prometheus + LB probes.
	auth.limiter.startJanitor(ctx)

	// CORS wraps auth so OPTIONS preflights bypass auth (browsers do not
	// send credentials on preflight). corsMiddleware short-circuits OPTIONS
	// before calling its `next`.
	protected := func(route string, h http.HandlerFunc) http.HandlerFunc {
		return corsMiddleware(auth.middleware(route, h))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/workers", protected("/api/workers", b.handleGetWorkers))
	mux.HandleFunc("/api/execute", protected("/api/execute", b.handleExecuteTask))
	mux.HandleFunc("/api/execute/async", protected("/api/execute/async", b.asyncExecuteHandler(ctx, &bgWG)))
	mux.HandleFunc("/v1/models", protected("/v1/models", b.handleModels))
	mux.HandleFunc("/v1/chat/completions", protected("/v1/chat/completions", b.handleChatCompletions))
	mux.HandleFunc("/v1/completions", protected("/v1/completions", b.handleCompletions))
	// /metrics and /health are intentionally not wrapped in CORS or auth —
	// Prometheus scrapers and LB health probes need neither, and exposing
	// CORS on /metrics would be misleading.
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health", b.handleHealth)

	srv := &http.Server{
		Addr:    net.JoinHostPort(bind, port),
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
		pterm.Success.Printfln("🚀 AgentFM API Gateway listening on http://%s", srv.Addr)
		if auth.tokens.empty() {
			pterm.Warning.Println("Auth disabled (AGENTFM_API_KEYS unset). Loopback bind keeps the gateway off the network.")
		} else {
			pterm.Info.Printfln("Auth enabled (%d API key(s) configured).", len(auth.tokens.tokens))
		}
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
		bgWG.Wait()
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

// handleHealth serves an unauthenticated liveness/readiness probe.
// Intended for load-balancer health checks and uptime monitors. Returns
// 200 + a tiny JSON body carrying the count of workers currently visible
// in telemetry — enough for an LB to also do shallow capacity-aware
// routing if it wants to.
func (b *Boss) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b.mu.RLock()
	online := len(b.activeWorkers)
	b.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"online_workers": online,
	}); err != nil {
		// Body already started; nothing useful to do.
		_ = err
	}
}

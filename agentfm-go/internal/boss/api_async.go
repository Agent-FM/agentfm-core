package boss

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/types"
	"agentfm/internal/version"

	"github.com/libp2p/go-libp2p/core/peer"
)

// webhookTimeout caps the POST we make to the client's callback URL. The
// default http.Client has no timeout at all, so a slow or hostile webhook
// would otherwise stall the async goroutine and block server shutdown.
const webhookTimeout = 30 * time.Second

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

		taskID := newCompletionID("task_")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"task_id": taskID,
			"status":  "queued",
			"message": "Task dispatched to P2P mesh.",
		}); err != nil {
			slog.Error("async task ack write", slog.Any(obs.FieldErr, err))
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

// runAsyncTask is the body of the background goroutine spawned by the
// async handler. It dials the worker, streams the prompt, drains the
// worker stdout into a scratch buffer, waits briefly for the artifact
// stream to land, then notifies the optional webhook URL.
func (b *Boss) runAsyncTask(ctx context.Context, peerID peer.ID, taskID string, req AsyncExecuteRequest) {
	started := time.Now()
	status := metrics.StatusError
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(started).Seconds())
		metrics.TasksTotal.WithLabelValues(status).Inc()
	}()

	s := b.dialOmni(ctx, peerID)
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
		slog.Error("async task set write deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}

	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    req.Prompt,
		TaskID:  taskID,
	}
	if err := json.NewEncoder(s).Encode(&payload); err != nil {
		slog.Error("async task send prompt", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}
	if err := s.CloseWrite(); err != nil {
		slog.Error("async task half-close", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}

	deadman := &timeoutReader{stream: s, timeout: network.TaskExecutionTimeout}
	var outputBuffer bytes.Buffer
	if _, err := io.Copy(&outputBuffer, deadman); err != nil {
		slog.Error("async task stream read", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID), slog.String(obs.FieldProtocol, "task"))
		return
	}

	streamSuccess = true
	status = metrics.StatusOK

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

	// SSRF / hostile-URL guard. Done at delivery time (rather than at submit)
	// so the operator's intent is observed against the URL we actually dial.
	if err := validateWebhookURL(req.WebhookURL); err != nil {
		slog.Warn("async task webhook rejected", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}

	webhookPayload := map[string]string{
		"task_id":   taskID,
		"worker_id": req.WorkerID,
		"status":    "completed",
	}
	jsonData, err := json.Marshal(webhookPayload)
	if err != nil {
		slog.Error("async task marshal webhook payload", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}

	// Bounded webhook POST. The default http.Client has NO timeout, so a
	// hostile or slow webhook URL would otherwise stall this goroutine
	// and therefore the server drain indefinitely.
	webhookCtx, webhookCancel := context.WithTimeout(ctx, webhookTimeout)
	defer webhookCancel()

	httpReq, err := http.NewRequestWithContext(webhookCtx, http.MethodPost, req.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		slog.Error("async task build webhook request", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if sig := signWebhookBody(jsonData); sig != "" {
		httpReq.Header.Set(signatureHeader, sig)
	}

	// safeWebhookClient closes the SSRF TOCTOU bypass: validateWebhookURL
	// resolved DNS at validation, but http.Client.Do would resolve again at
	// dial. The custom DialContext re-validates every resolved IP and
	// refuses private addresses, so a hostile DNS that returned public at
	// validation and private at dial cannot pivot into the metadata service.
	client := safeWebhookClient(webhookTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		slog.Error("async task webhook delivery", slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}
	// Drain a bounded slice of the body before closing. The webhook
	// contract is "ack only" — we never inspect the body. Without this
	// bound a hostile server can return a 100 GiB body and Close() does
	// not cancel the read; webhookCtx times out the connection setup,
	// not the body read after headers arrived.
	_, _ = io.CopyN(io.Discard, resp.Body, MaxWebhookResponseBytes)
	_ = resp.Body.Close()
}

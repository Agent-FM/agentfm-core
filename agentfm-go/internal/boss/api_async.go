package boss

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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

// asyncArtifactWait is the upper bound on how long runAsyncTask waits for
// the artifact zip to appear on disk before firing the webhook. Workers
// that produce no artifacts (the [AGENTFM: NO_FILES] sentinel) hit this
// timeout and proceed; workers that DO produce artifacts usually deliver
// them within hundreds of milliseconds via HandleArtifactStream.
const asyncArtifactWait = 10 * time.Second

// waitForArtifact polls for agentfm_artifacts/<taskID>.zip with a 100ms
// cadence until the file appears or maxWait elapses or ctx is cancelled.
// Returns whether the file was observed (currently informational only;
// callers fire the webhook either way).
func waitForArtifact(ctx context.Context, taskID string, maxWait time.Duration) bool {
	// Defense-in-depth: today the only caller passes a newCompletionID()
	// (crypto/rand hex), so a path-traversal payload is impossible. But
	// the helper is generic; refuse anything that can't safely be joined
	// into a filesystem path so a future caller can't accidentally
	// inherit a traversal bug.
	if !network.SafeTaskIDPattern.MatchString(taskID) {
		slog.Warn("waitForArtifact rejected unsafe taskID",
			slog.String(obs.FieldTaskID, taskID),
		)
		return false
	}
	deadline := time.Now().Add(maxWait)
	zipPath := filepath.Join("agentfm_artifacts", taskID+".zip")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(zipPath); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
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

		taskID := newCompletionID("task_")

		// Acquire an async slot non-blockingly. When MaxInflightAsyncTasks
		// is exhausted, return 503 with a Retry-After hint instead of
		// silently spawning unbounded goroutines (DoS vector). The slot
		// is released by the goroutine on exit.
		select {
		case b.asyncSlots <- struct{}{}:
		default:
			w.Header().Set("Retry-After", "5")
			writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError,
				"async_capacity_exhausted",
				"too many async tasks in flight; retry shortly")
			return
		}

		// Spawn-before-ack: the goroutine starts BEFORE we attempt to
		// write the 202 body. This preserves the contract "if the client
		// got a 202 with a task_id, the task is being executed." A failed
		// ack write (client hung up between header and body) leaves the
		// background task running so a webhook delivery can still fire,
		// rather than silently dropping committed work.
		taskCtx, cancelTask := context.WithTimeout(rootCtx, network.TaskExecutionTimeout)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancelTask()
			defer func() { <-b.asyncSlots }()

			b.runAsyncTask(taskCtx, peerID, taskID, req)
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"task_id": taskID,
			"status":  "queued",
			"message": "Task dispatched to P2P mesh.",
		}); err != nil {
			slog.Warn("async task ack write failed; goroutine continues",
				slog.String(obs.FieldTaskID, taskID),
				slog.Any(obs.FieldErr, err),
			)
		}
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

	// Wait for the out-of-band artifact stream to land on disk before
	// notifying the webhook. The artifact zip lives at
	// agentfm_artifacts/<taskID>.zip; poll for it with a bounded deadline.
	// If the file appears earlier we fire immediately (responsive UX);
	// if it doesn't appear within asyncArtifactWait we fire anyway with
	// only the stdout result (the worker may have legitimately produced
	// no artifacts via the [AGENTFM: NO_FILES] sentinel).
	waitForArtifact(ctx, taskID, asyncArtifactWait)

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

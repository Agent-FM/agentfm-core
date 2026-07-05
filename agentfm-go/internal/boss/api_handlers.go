package boss

import (
	"context"
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

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

// handleGetWorkers serves the /api/workers listing as a pure read.
// Eviction of disconnected peers happens on a 30s tick inside
// listenTelemetry (pruneDisconnectedWorkers); GETs are no-side-effect.
//
// Optional query parameter:
//   - ?include_offline=true: also includes peers that only appear in
//     ledger entries (gossipped via inbox or own log) but are not
//     currently connected. Enables the operator radar to surface
//     offline peers and their trust scores.
func (b *Boss) handleGetWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	includeOffline := r.URL.Query().Get("include_offline") == "true"

	// Fetch merged online + (optionally) offline peer list.
	known, err := b.ListKnownPeers(r.Context())
	if err != nil {
		// Fall back to active-only on store error.
		slog.Warn("known-peers query failed; serving active-only", slog.Any(obs.FieldErr, err))
		known = nil
	}

	agents := make([]apiWorker, 0, len(known))
	onlineCount, offlineCount := 0, 0

	for _, kp := range known {
		if !kp.IsOnline && !includeOffline {
			continue
		}
		if kp.IsOnline {
			onlineCount++
		} else {
			offlineCount++
		}

		// Pull cached profile if available (online → in activeWorkers; offline → empty stub).
		// Use PeerIDStr (the original map key) rather than PeerID.String() so raw-string
		// keys (e.g. legacy or test-injected IDs) resolve correctly.
		b.mu.RLock()
		profile, hasProfile := b.activeWorkers[kp.PeerIDStr]
		b.mu.RUnlock()
		if !hasProfile {
			profile = types.WorkerProfile{PeerID: kp.PeerIDStr}
		}

		var lastSeenPtr *time.Time
		if !kp.LastSeen.IsZero() {
			ls := kp.LastSeen
			lastSeenPtr = &ls
		}
		aw := b.profileToAPIWorker(profile)
		aw.Online = kp.IsOnline
		aw.LastSeen = lastSeenPtr
		aw.HonestyScore = kp.HonestyScore
		aw.IsEquivocator = kp.IsEquivocator
		agents = append(agents, aw)
	}

	response := map[string]any{
		"success":       true,
		"online_count":  onlineCount,
		"offline_count": offlineCount,
		"agents":        agents,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("encode /api/workers response", slog.Any(obs.FieldErr, err))
	}
}

// computeTrustView derives the visibility trust fields for a peer.
// Called by both profileToAPIWorker and profileToModelEntry so the
// logic isn't duplicated across two conversion helpers.
//
// Phase 1 logic: equivocator → dispatch blocked; else allowed.
// Phase 8 will add the reputation-floor check here.
func (b *Boss) computeTrustView(peerIDStr string) (honesty float64, equivocator bool, dispatchAllowed bool, refuseReason string) {
	dispatchAllowed = true
	if b.ledger != nil {
		pid, err := peer.Decode(peerIDStr)
		if err == nil {
			marked, ierr := b.ledger.IsEquivocator(context.Background(), []byte(pid))
			if ierr == nil && marked {
				equivocator = true
				dispatchAllowed = false
				refuseReason = "peer_is_equivocator"
			}
		}
	}
	if b.reputationEngine != nil {
		honesty = b.reputationEngine.Score(peerIDStr)
	}
	return
}

// profileToAPIWorker is the method-on-Boss form of the old standalone
// profileToAPIWorker function. It now populates the visibility fields
// (image, capability, honesty, equivocator, dispatch_allowed) by calling
// computeTrustView.
func (b *Boss) profileToAPIWorker(p types.WorkerProfile) apiWorker {
	hardwareStr := fmt.Sprintf("%s (CPU: %d Cores)", p.Model, p.CPUCores)
	if p.HasGPU {
		hardwareStr = fmt.Sprintf("%s (GPU VRAM: %.1f/%.1f GB)", p.Model, p.GPUUsedGB, p.GPUTotalGB)
	}
	honesty, equivocator, dispatchAllowed, refuseReason := b.computeTrustView(p.PeerID)
	return apiWorker{
		PeerID:               p.PeerID,
		Author:               p.Author,
		Name:                 p.AgentName,
		Status:               p.Status,
		Hardware:             hardwareStr,
		Description:          p.AgentDesc,
		CPUUsagePct:          p.CPUUsagePct,
		RAMFreeGB:            p.RAMFreeGB,
		CurrentTasks:         p.CurrentTasks,
		MaxTasks:             p.MaxTasks,
		HasGPU:               p.HasGPU,
		GPUUsedGB:            p.GPUUsedGB,
		GPUTotalGB:           p.GPUTotalGB,
		GPUUsagePct:          p.GPUUsagePct,
		AgentImageRef:        p.AgentImageRef,
		AgentImageDigest:     p.AgentImageDigest,
		AgentCapability:      p.AgentCapability,
		HonestyScore:         honesty,
		IsEquivocator:        equivocator,
		DispatchAllowed:      dispatchAllowed,
		DispatchRefuseReason: refuseReason,
		Online:               true, // all activeWorkers are live peers
		LastSeen:             nil,  // populated in Phase 6
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
	if !network.SafeTaskIDPattern.MatchString(req.TaskID) {
		http.Error(w, "Invalid task_id format", http.StatusBadRequest)
		return
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

	pterm.Info.Printfln("📡 API Gateway routing task %s to Worker %s...",
		shortID(req.TaskID, 8), pterm.Cyan(peerID.String()[:8]))

	// Tie the dial to the inbound HTTP request's context so a client
	// hanging up aborts the libp2p dial instead of waiting out the full
	// StreamDialTimeout. Use dialWorkerStream (spinner-free) here — the
	// HTTP path must not spawn a TUI spinner, which carries a known
	// concurrent-state race inside pterm's SpinnerPrinter goroutine.
	b.expectArtifact(req.TaskID, peerID)

	s, dialErr := b.dialWorkerStream(r.Context(), peerID)
	if dialErr != nil {
		if b.completionRater != nil {
			b.completionRater.RecordOutcome(peerID, OutcomeFailure)
		}
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

	// Record exactly one outcome per dispatch attempt after the stream is
	// established. streamSuccess is false on every failure path; the
	// deferred recorder below fires unconditionally.
	defer func() {
		if b.completionRater == nil {
			return
		}
		if streamSuccess {
			b.completionRater.RecordOutcome(peerID, OutcomeSuccess)
		} else {
			b.completionRater.RecordOutcome(peerID, OutcomeFailure)
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

	// Best-effort: persist optional feedback comment + rating to the ledger.
	// The task already succeeded; a feedback-append failure only gets logged.
	if req.Feedback != "" && b.completionRater != nil {
		if ferr := b.appendFeedbackComment(r.Context(), peerID, req.TaskID, req.Feedback, req.FeedbackRating); ferr != nil {
			slog.Warn("feedback persist failed", slog.Any(obs.FieldErr, ferr), slog.String(obs.FieldTaskID, req.TaskID))
		}
	}
}

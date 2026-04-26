package boss

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (b *Boss) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, errCodeMethodNotAllowed, "POST only")
		return
	}

	var req CompletionRequest
	limited := io.LimitReader(r.Body, 1*1024*1024)
	if err := json.NewDecoder(limited).Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, errCodeInvalidRequest, "invalid JSON payload")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, errCodeModelRequired, "field 'model' is required")
		return
	}

	prompt, perr := promptString(req.Prompt)
	if perr != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, perr.code, perr.msg)
		return
	}

	worker, err := b.pickWorker(req.Model)
	if err != nil {
		writeOpenAIWorkerError(w, req.Model, err)
		return
	}

	peerID, derr := peer.Decode(worker.PeerID)
	if derr != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, errCodeInternalError, "selected worker has malformed peer id")
		return
	}

	taskID := newCompletionID("task_")

	if req.Stream {
		b.streamTextCompletion(r.Context(), w, peerID, req.Model, prompt, taskID)
		return
	}

	ts := b.openTaskStream(r.Context(), w, peerID, prompt, taskID)
	if ts == nil {
		return
	}
	defer ts.close()

	text, err := drainTaskStream(ts.s)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerStreamFailed, "worker stream read failed: "+err.Error())
		return
	}
	ts.success = true

	resp := completionResponse{
		ID:      newCompletionID("cmpl-"),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []completionChoice{{
			Index:        0,
			Text:         text,
			FinishReason: "stop",
		}},
		Usage: usage{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encode /v1/completions response", slog.Any(obs.FieldErr, err))
	}
}

func (b *Boss) streamTextCompletion(ctx context.Context, w http.ResponseWriter, peerID peer.ID, model, prompt, taskID string) {
	ts := b.openTaskStream(ctx, w, peerID, prompt, taskID)
	if ts == nil {
		return
	}
	defer ts.close()

	flush := setSSEHeaders(w)
	id := newCompletionID("cmpl-")
	created := time.Now().Unix()

	emit := func(text string, finish *string) bool {
		return writeSSEFrame(w, completionChunk{
			ID:      id,
			Object:  "text_completion",
			Created: created,
			Model:   model,
			Choices: []completionChoiceDelta{{Index: 0, Text: text, FinishReason: finish}},
		}, flush)
	}

	scanner := newTaskStreamScanner(ts.s)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !emit(scanner.Text()+"\n", nil) {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		stop := "stop"
		_ = emit("\n[stream error: "+err.Error()+"]\n", &stop)
		writeSSEDone(w, flush)
		return
	}

	ts.success = true

	stop := "stop"
	_ = emit("", &stop)
	writeSSEDone(w, flush)
}

type promptError struct {
	code string
	msg  string
}

func (p *promptError) Error() string { return p.msg }

func promptString(prompt any) (string, *promptError) {
	switch v := prompt.(type) {
	case nil:
		return "", &promptError{code: errCodePromptRequired, msg: "field 'prompt' is required"}
	case string:
		if strings.TrimSpace(v) == "" {
			return "", &promptError{code: errCodePromptRequired, msg: "field 'prompt' must not be empty"}
		}
		return v, nil
	case []any:
		return "", &promptError{code: errCodeUnsupportedPrompt, msg: "array form of 'prompt' is not supported in this build; pass a single string"}
	default:
		return "", &promptError{code: errCodeUnsupportedPrompt, msg: "field 'prompt' must be a string"}
	}
}

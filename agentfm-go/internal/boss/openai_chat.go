package boss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

func (b *Boss) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, errCodeMethodNotAllowed, "POST only")
		return
	}

	var req ChatCompletionRequest
	limited := io.LimitReader(r.Body, 1*1024*1024)
	if err := json.NewDecoder(limited).Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, errCodeInvalidRequest, "invalid JSON payload")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, errCodeModelRequired, "field 'model' is required")
		return
	}
	if len(req.Messages) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, errTypeInvalidRequest, errCodePromptRequired, "field 'messages' must contain at least one message")
		return
	}

	worker, err := b.pickWorker(req.Model)
	if err != nil {
		writeOpenAIWorkerError(w, req.Model, err)
		return
	}

	peerID, perr := peer.Decode(worker.PeerID)
	if perr != nil {
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, errCodeInternalError, "selected worker has malformed peer id")
		return
	}

	prompt := renderChatPrompt(req.Messages)
	taskID := newCompletionID("task_")

	if req.Stream {
		b.streamChatCompletion(r.Context(), w, peerID, req.Model, prompt, taskID)
		return
	}

	ts := b.openTaskStream(r.Context(), w, peerID, prompt, taskID)
	if ts == nil {
		return
	}
	defer ts.close()

	content, err := drainTaskStream(ts.s)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerStreamFailed, "worker stream read failed: "+err.Error())
		return
	}
	ts.success = true

	resp := chatCompletionResponse{
		ID:      newCompletionID("chatcmpl-"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []chatChoice{{
			Index:        0,
			Message:      ChatMessage{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
		Usage: usage{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		pterm.Error.Printfln("Failed to encode /v1/chat/completions response: %v", err)
	}
}

func (b *Boss) streamChatCompletion(ctx context.Context, w http.ResponseWriter, peerID peer.ID, model, prompt, taskID string) {
	ts := b.openTaskStream(ctx, w, peerID, prompt, taskID)
	if ts == nil {
		return
	}
	defer ts.close()

	flush := setSSEHeaders(w)
	id := newCompletionID("chatcmpl-")
	created := time.Now().Unix()

	emit := func(delta ChatMessage, finish *string) bool {
		return writeSSEFrame(w, chatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   model,
			Choices: []chatChoiceDelta{{Index: 0, Delta: delta, FinishReason: finish}},
		}, flush)
	}

	if !emit(ChatMessage{Role: "assistant"}, nil) {
		return
	}

	scanner := newTaskStreamScanner(ts.s)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !emit(ChatMessage{Content: scanner.Text() + "\n"}, nil) {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		stop := "stop"
		_ = emit(ChatMessage{Content: "\n[stream error: " + err.Error() + "]\n"}, &stop)
		writeSSEDone(w, flush)
		return
	}

	ts.success = true

	stop := "stop"
	_ = emit(ChatMessage{}, &stop)
	writeSSEDone(w, flush)
}

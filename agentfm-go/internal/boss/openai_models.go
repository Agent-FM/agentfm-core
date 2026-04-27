package boss

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"agentfm/internal/obs"
)

func (b *Boss) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, errCodeMethodNotAllowed, "GET only")
		return
	}

	now := time.Now().Unix()
	b.mu.RLock()
	data := make([]modelEntry, 0, len(b.activeWorkers))
	for _, p := range b.activeWorkers {
		if p.PeerID == "" {
			continue
		}
		data = append(data, profileToModelEntry(p, now))
	}
	b.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelsResponse{Object: "list", Data: data}); err != nil {
		slog.Error("encode /v1/models response", slog.Any(obs.FieldErr, err))
	}
}

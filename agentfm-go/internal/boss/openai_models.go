package boss

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"agentfm/internal/obs"
	"agentfm/internal/types"
)

func (b *Boss) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, errCodeMethodNotAllowed, "GET only")
		return
	}

	now := time.Now().Unix()
	b.mu.RLock()
	profiles := make([]types.WorkerProfile, 0, len(b.activeWorkers))
	for _, p := range b.activeWorkers {
		if p.PeerID == "" {
			continue
		}
		profiles = append(profiles, p)
	}
	b.mu.RUnlock()

	data := make([]modelEntry, 0, len(profiles))
	for _, p := range profiles {
		data = append(data, b.profileToModelEntry(r.Context(), p, now))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelsResponse{Object: "list", Data: data}); err != nil {
		slog.Error("encode /v1/models response", slog.Any(obs.FieldErr, err))
	}
}

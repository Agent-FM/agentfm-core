package boss

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pterm/pterm"
)

func (b *Boss) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, errTypeInvalidRequest, errCodeMethodNotAllowed, "GET only")
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now().Unix()
	data := make([]modelEntry, 0, len(b.activeWorkers))
	for _, p := range b.activeWorkers {
		if p.PeerID == "" {
			continue
		}
		data = append(data, profileToModelEntry(p, now))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelsResponse{Object: "list", Data: data}); err != nil {
		pterm.Error.Printfln("Failed to encode /v1/models response: %v", err)
	}
}

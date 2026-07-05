package boss

import (
	"encoding/json"
	"os"
	"path/filepath"

	"agentfm/internal/network"
	"agentfm/internal/obs"

	"log/slog"

	"github.com/libp2p/go-libp2p/core/peer"
)

type artifactMetaSidecar struct {
	AgentName        string `json:"agentName,omitempty"`
	AgentDescription string `json:"agentDescription,omitempty"`
	AgentPeerID      string `json:"agentPeerId,omitempty"`
}

func (b *Boss) writeArtifactIdentitySidecar(taskID string, worker peer.ID) {
	if !network.SafeTaskIDPattern.MatchString(taskID) {
		return
	}

	workerStr := worker.String()
	b.mu.RLock()
	profile, ok := b.activeWorkers[workerStr]
	if !ok {
		profile, ok = b.lastProfile[workerStr]
	}
	b.mu.RUnlock()
	if !ok || profile.AgentName == "" {
		return
	}

	meta := artifactMetaSidecar{
		AgentName:        profile.AgentName,
		AgentDescription: profile.AgentDesc,
		AgentPeerID:      workerStr,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return
	}

	dir := "agentfm_artifacts"
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("artifact meta sidecar: mkdir",
			slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
		return
	}
	path := filepath.Join(dir, taskID+".meta.json")
	if _, statErr := os.Stat(path); statErr == nil {
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		slog.Warn("artifact meta sidecar: write",
			slog.Any(obs.FieldErr, err), slog.String(obs.FieldTaskID, taskID))
	}
}

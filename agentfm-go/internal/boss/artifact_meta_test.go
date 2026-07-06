package boss

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestWriteArtifactIdentitySidecar_UsesWorkerProfile(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	worker := peer.ID("worker-peer-id-bytes")
	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{
			worker.String(): {PeerID: worker.String(), AgentName: "HR Agent", AgentDesc: "drafts HR emails"},
		},
		lastProfile: map[string]types.WorkerProfile{},
	}

	taskID := "task_meta123"
	b.writeArtifactIdentitySidecar(taskID, worker)

	data, err := os.ReadFile(filepath.Join("agentfm_artifacts", taskID+".meta.json"))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	var m struct {
		AgentName        string `json:"agentName"`
		AgentPeerID      string `json:"agentPeerId"`
		AgentDescription string `json:"agentDescription"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.AgentName != "HR Agent" {
		t.Errorf("agentName = %q, want %q", m.AgentName, "HR Agent")
	}
	if m.AgentPeerID != worker.String() {
		t.Errorf("agentPeerId = %q, want %q", m.AgentPeerID, worker.String())
	}
	if m.AgentDescription != "drafts HR emails" {
		t.Errorf("agentDescription = %q, want %q", m.AgentDescription, "drafts HR emails")
	}
}

func TestWriteArtifactIdentitySidecar_FallsBackToLastProfile(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	worker := peer.ID("offline-worker-bytes")
	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{},
		lastProfile: map[string]types.WorkerProfile{
			worker.String(): {PeerID: worker.String(), AgentName: "Cached Agent"},
		},
	}

	taskID := "task_meta456"
	b.writeArtifactIdentitySidecar(taskID, worker)

	data, err := os.ReadFile(filepath.Join("agentfm_artifacts", taskID+".meta.json"))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	var m struct {
		AgentName string `json:"agentName"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.AgentName != "Cached Agent" {
		t.Errorf("agentName = %q, want %q", m.AgentName, "Cached Agent")
	}
}

func TestWriteArtifactIdentitySidecar_UnknownWorkerWritesNothing(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	b := &Boss{
		activeWorkers: map[string]types.WorkerProfile{},
		lastProfile:   map[string]types.WorkerProfile{},
	}

	taskID := "task_meta789"
	b.writeArtifactIdentitySidecar(taskID, peer.ID("nobody"))

	if _, err := os.Stat(filepath.Join("agentfm_artifacts", taskID+".meta.json")); !os.IsNotExist(err) {
		t.Errorf("expected no sidecar for unknown worker, stat err = %v", err)
	}
}

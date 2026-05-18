package types

type WorkerProfile struct {
	PeerID       string  `json:"peer_id"`
	CPUCores     int     `json:"cpu_cores"`
	CPUUsagePct  float64 `json:"cpu_usage_pct"`
	RAMFreeGB    float64 `json:"ram_free_gb"`
	Model        string  `json:"model"`
	AgentName    string  `json:"agent_name"`
	AgentDesc    string  `json:"agent_desc"`
	Status       string  `json:"status"`
	HasGPU       bool    `json:"has_gpu"`
	GPUUsedGB    float64 `json:"gpu_used_gb"`
	GPUTotalGB   float64 `json:"gpu_total_gb"`
	GPUUsagePct  float64 `json:"gpu_usage_pct"`
	CurrentTasks int     `json:"current_tasks"`
	MaxTasks     int     `json:"max_tasks"`
	Author       string  `json:"author"`

	// IsWitness reports whether this peer offers the P2-2 witness
	// co-sign service. Relay nodes default this to true; plain
	// workers default false. Boss matchers and ledger client code
	// use this to discover witnesses without a separate registry.
	IsWitness bool `json:"is_witness,omitempty"`

	// AgentImageDigest is the OCI manifest digest of the Podman
	// image this worker advertises running. Empty when the worker
	// has not been able to resolve a digest. 32-byte raw SHA-256
	// hex-encoded as "sha256:...".
	AgentImageDigest string `json:"agent_image_digest,omitempty"`

	// AgentImageRef is the human-readable image reference the
	// worker was launched with (e.g. "ghcr.io/agentfm/sick-leave:v1").
	AgentImageRef string `json:"agent_image_ref,omitempty"`

	// AgentCapability is a kebab-case capability tag the worker
	// claims (e.g. "hr-specialist", "code-helper"). Used by future
	// probe coordinators (v1.4) to match probes to agents. v1.3
	// boss records but does not act on it.
	AgentCapability string `json:"agent_capability,omitempty"`
}

// TaskPayload is the JSON envelope the Boss sends to a Worker over
// TaskProtocol. Shared between both sides so there is one schema and no
// hand-rolled JSON formatting in the boss package.
type TaskPayload struct {
	Version string `json:"version"`
	Task    string `json:"task"`
	Data    string `json:"data"`
	TaskID  string `json:"task_id"`
}

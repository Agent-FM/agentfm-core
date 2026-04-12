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
}

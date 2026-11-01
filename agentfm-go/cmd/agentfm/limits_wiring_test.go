package main

import (
	"testing"

	"agentfm/internal/worker"
)

// The -task-* flags travel main() -> validateOperatorConfig -> worker.Config ->
// SandboxSpec -> podman argv. Every hop but the first was covered by tests in
// internal/worker, which set Config.Limits directly — so a version of
// validateOperatorConfig that took its Config by value, silently discarding
// the ceilings it had just computed, passed the entire suite while the worker
// ran completely unbounded and its documentation promised otherwise.
//
// These tests exist to make that specific failure impossible to reintroduce.

func TestValidateOperatorConfig_PropagatesLimitsToConfig(t *testing.T) {
	cfg := worker.Config{MaxCPU: 80, MaxGPU: 80, MaxConcurrentTasks: 1}

	validateOperatorConfig(&cfg, 2, "512m", 777)

	if cfg.Limits.CPUs != 2 {
		t.Errorf("CPUs = %v, want 2 — the -task-cpus flag never reached the Config", cfg.Limits.CPUs)
	}
	if cfg.Limits.MemoryBytes != 512<<20 {
		t.Errorf("MemoryBytes = %d, want %d — the -task-memory flag never reached the Config",
			cfg.Limits.MemoryBytes, 512<<20)
	}
	if cfg.Limits.PidsLimit != 777 {
		t.Errorf("PidsLimit = %d, want 777 — the -task-pids-limit flag never reached the Config", cfg.Limits.PidsLimit)
	}
}

// The ceilings the Config carries must be exactly what the container is given.
// This closes the loop end to end: flag values in, podman argv out.
func TestValidateOperatorConfig_LimitsReachTheContainerArgv(t *testing.T) {
	cfg := worker.Config{MaxCPU: 80, MaxGPU: 80, MaxConcurrentTasks: 1}
	validateOperatorConfig(&cfg, 2, "512m", 777)

	args, err := worker.BuildRunArgs(worker.SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		OutputDir:     "/out",
		Network:       worker.NetworkHost,
		Limits:        cfg.Limits,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs: %v", err)
	}

	want := map[string]string{
		"--cpus":        "2",
		"--memory":      "536870912",
		"--memory-swap": "536870912",
		"--pids-limit":  "777",
	}
	for flag, value := range want {
		var found bool
		for i, a := range args {
			if a == flag && i+1 < len(args) && args[i+1] == value {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s %s missing from the container argv: %q", flag, value, args)
		}
	}
}

// The default must actually be applied, not merely declared: an operator who
// passes no -task-pids-limit still gets fork-bomb containment.
func TestValidateOperatorConfig_AppliesDefaultPidsLimit(t *testing.T) {
	cfg := worker.Config{MaxCPU: 80, MaxGPU: 80, MaxConcurrentTasks: 1}

	validateOperatorConfig(&cfg, 0, "", worker.DefaultPidsLimit)

	if cfg.Limits.PidsLimit != worker.DefaultPidsLimit {
		t.Errorf("PidsLimit = %d, want the default %d", cfg.Limits.PidsLimit, worker.DefaultPidsLimit)
	}
	if cfg.Limits.CPUs != 0 || cfg.Limits.MemoryBytes != 0 {
		t.Errorf("CPU and memory must stay unlimited by default, got %+v", cfg.Limits)
	}
}

// -sandbox-network travels the same path as the -task-* ceilings, so it gets
// the same end-to-end check: flag value and swarm key in, podman argv out. The
// public-mesh case is the one that matters — an operator who passes nothing
// and no -swarmkey must get --network none.
func TestResolveSandboxNetwork_ReachesTheContainerArgv(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		swarmKey string
		want     string
	}{
		{"public mesh default", "", "", "none"},
		{"private swarm default", "", "./swarm.key", "host"},
		{"public mesh explicit host", "host", "", "host"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := worker.Config{}
			if err := resolveSandboxNetwork(&cfg, tc.flag, tc.swarmKey); err != nil {
				t.Fatalf("resolveSandboxNetwork: %v", err)
			}

			args, err := worker.BuildRunArgs(worker.SandboxSpec{
				ContainerName: "c",
				Image:         "img",
				OutputDir:     "/out",
				Network:       cfg.Network,
			})
			if err != nil {
				t.Fatalf("BuildRunArgs: %v", err)
			}

			var got string
			for i, a := range args {
				if a == "--network" && i+1 < len(args) {
					got = args[i+1]
				}
			}
			if got != tc.want {
				t.Errorf("--network = %q, want %q (argv %q)", got, tc.want, args)
			}
		})
	}
}

func TestResolveSandboxNetwork_RejectsUnknownValue(t *testing.T) {
	cfg := worker.Config{}
	if err := resolveSandboxNetwork(&cfg, "bridge", ""); err == nil {
		t.Fatal("resolveSandboxNetwork accepted \"bridge\"; want an error")
	}
	if cfg.Network != "" {
		t.Errorf("cfg.Network = %q after a rejected value; want it left unset", cfg.Network)
	}
}

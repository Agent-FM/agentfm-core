package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"

	"github.com/pterm/pterm"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

func (w *Worker) truncateWords(text string, maxWords int) string {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text
	}
	return strings.Join(words[:maxWords], " ") + "..."
}

func getGPUStats() (hasGPU bool, usedGB float64, totalGB float64, usagePct float64) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return false, 0, 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return false, 0, 0, 0
	}
	parts := strings.Split(lines[0], ",")
	if len(parts) < 2 {
		return false, 0, 0, 0
	}

	// If either number fails to parse the nvidia-smi output is malformed;
	// treat the probe as a miss rather than publishing garbage telemetry.
	usedMB, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return false, 0, 0, 0
	}
	totalMB, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return false, 0, 0, 0
	}

	usedGB = usedMB / 1024.0
	totalGB = totalMB / 1024.0
	if totalGB > 0 {
		usagePct = (usedGB / totalGB) * 100.0
	}
	return true, usedGB, totalGB, usagePct
}

func (w *Worker) printMetadata() {
	pterm.Info.Printfln("Peer ID:  %s", pterm.Gray(w.node.Host.ID().String()))
	pterm.Info.Printfln("Agent:    %s", pterm.LightMagenta(w.config.AgentName))
	pterm.Info.Printfln("Model:    %s", pterm.LightYellow(w.config.ModelName))
	pterm.Info.Printfln("Author:   %s", pterm.LightCyan(w.config.Author))
	pterm.Info.Printfln("Capacity: %s", pterm.LightCyan(fmt.Sprintf("%d tasks | %.0f%% Max CPU | %.0f%% Max GPU", w.config.MaxConcurrentTasks, w.config.MaxCPU, w.config.MaxGPU)))
	fmt.Println()
}

func (w *Worker) startTelemetry(ctx context.Context) {
	topic, err := w.node.PubSub.Join(network.TelemetryTopic)
	if err != nil {
		// Non-fatal: surface the error and return so parent defers still
		// run. The worker continues to serve tasks but won't appear on
		// the radar until it's restarted.
		pterm.Error.Printfln("Telemetry disabled: failed to join %q topic: %v", network.TelemetryTopic, err)
		return
	}
	defer func() { _ = topic.Close() }()

	safeDesc := w.truncateWords(w.config.AgentDesc, 50)
	totalCores := runtime.NumCPU()

	// Sensor / publish errors are noisy if logged every tick. Track the
	// last logged message so we only surface changes, not steady-state
	// failures.
	var lastSensorErr, lastPublishErr string

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cpuPercent, cpuErr := cpu.Percent(time.Second, false)
			if cpuErr != nil {
				if msg := cpuErr.Error(); msg != lastSensorErr {
					pterm.Warning.Printfln("cpu sensor read failed: %v", cpuErr)
					lastSensorErr = msg
				}
			}

			hasGPU, gpuUsed, gpuTotal, gpuPct := getGPUStats()

			w.mu.Lock()
			if len(cpuPercent) > 0 {
				w.currentCPU = cpuPercent[0]
			}
			snapCPU := w.currentCPU
			activeTasks := w.currentTasks
			maxTasks := w.config.MaxConcurrentTasks
			w.mu.Unlock()

			status := "AVAILABLE"
			if activeTasks >= maxTasks {
				status = "BUSY"
			} else if snapCPU >= w.config.MaxCPU { // Dynamic Threshold
				status = "BUSY"
			} else if hasGPU && gpuPct > w.config.MaxGPU { // Dynamic Threshold
				status = "BUSY"
			}

			vMem, memErr := mem.VirtualMemory()
			if memErr != nil {
				if msg := memErr.Error(); msg != lastSensorErr {
					pterm.Warning.Printfln("memory sensor read failed: %v", memErr)
					lastSensorErr = msg
				}
			}
			freeRAM := 0.0
			if vMem != nil {
				freeRAM = float64(vMem.Available) / (1024 * 1024 * 1024)
			}

			profile := types.WorkerProfile{
				PeerID:       w.node.Host.ID().String(),
				Author:       w.config.Author,
				CPUCores:     totalCores,
				CPUUsagePct:  snapCPU,
				RAMFreeGB:    freeRAM,
				Model:        w.config.ModelName,
				AgentName:    w.config.AgentName,
				AgentDesc:    safeDesc,
				Status:       status,
				HasGPU:       hasGPU,
				GPUUsedGB:    gpuUsed,
				GPUTotalGB:   gpuTotal,
				GPUUsagePct:  gpuPct,
				CurrentTasks: activeTasks,
				MaxTasks:     maxTasks,
			}

			payloadBytes, marshalErr := json.Marshal(profile)
			if marshalErr != nil {
				pterm.Error.Printfln("failed to marshal telemetry profile: %v", marshalErr)
				continue
			}
			if pubErr := topic.Publish(ctx, payloadBytes); pubErr != nil {
				if msg := pubErr.Error(); msg != lastPublishErr {
					pterm.Warning.Printfln("telemetry publish failed: %v", pubErr)
					lastPublishErr = msg
				}
			} else {
				lastPublishErr = ""
			}

			if status == "BUSY" {
				if hasGPU {
					pterm.Warning.Printfln("GOSSIP | Tasks: %d/%d | CPU: %4.1f%% | GPU VRAM: %4.1f%% | Status: %s", activeTasks, maxTasks, profile.CPUUsagePct, profile.GPUUsagePct, pterm.Red("🔴 BUSY"))
				} else {
					pterm.Warning.Printfln("GOSSIP | Tasks: %d/%d | CPU: %4.1f%% | Status: %s", activeTasks, maxTasks, profile.CPUUsagePct, pterm.Red("🔴 BUSY"))
				}
			} else {
				pterm.Success.Printfln("GOSSIP | Tasks: %d/%d | CPU: %4.1f%% | Status: %s", activeTasks, maxTasks, profile.CPUUsagePct, pterm.Green("🟢 AVAILABLE"))
			}
		}
	}
}

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

func getGPUStats() (hasGPU bool, usedGB float64, totalGB float64, usagePct float64) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return false, 0, 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		parts := strings.Split(lines[0], ",")
		if len(parts) >= 2 {
			usedMB, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			totalMB, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

			usedGB = usedMB / 1024.0
			totalGB = totalMB / 1024.0
			if totalGB > 0 {
				usagePct = (usedGB / totalGB) * 100.0
			}
			return true, usedGB, totalGB, usagePct
		}
	}
	return false, 0, 0, 0
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
		pterm.Fatal.Println(err)
	}

	safeDesc := w.truncateWords(w.config.AgentDesc, 50)
	totalCores := runtime.NumCPU()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cpuPercent, _ := cpu.Percent(time.Second, false)

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

			vMem, _ := mem.VirtualMemory()
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

			payloadBytes, _ := json.Marshal(profile)
			topic.Publish(ctx, payloadBytes)

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

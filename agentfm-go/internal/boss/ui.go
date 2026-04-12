package boss

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

func renderCPUBar(percent float64) string {
	barsTotal := 10
	filled := int((percent / 100.0) * float64(barsTotal))
	if filled > barsTotal {
		filled = barsTotal
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barsTotal-filled)

	if percent >= 80 {
		return pterm.Red(fmt.Sprintf("[%s] %5.1f%%", bar, percent))
	} else if percent >= 50 {
		return pterm.Yellow(fmt.Sprintf("[%s] %5.1f%%", bar, percent))
	}
	return pterm.Green(fmt.Sprintf("[%s] %5.1f%%", bar, percent))
}

func (b *Boss) selectWorkerInteractive() (types.WorkerProfile, bool) {
	uiCtx, cancelUI := context.WithCancel(context.Background())
	defer cancelUI()

	selectedIndex := 0
	var displayList []types.WorkerProfile

	area, _ := pterm.DefaultArea.WithFullscreen(true).Start()
	defer area.Stop()

	draw := func() {
		b.mu.Lock()

		for peerID, seen := range b.lastSeen {
			if time.Since(seen) > 15*time.Second {
				delete(b.activeWorkers, peerID)
				delete(b.lastSeen, peerID)
			}
		}

		displayList = make([]types.WorkerProfile, 0, len(b.activeWorkers))
		for _, w := range b.activeWorkers {
			displayList = append(displayList, w)
		}

		activeCount := len(b.activeWorkers)
		peerCount := len(b.node.PubSub.ListPeers(network.TelemetryTopic))

		b.mu.Unlock()

		sort.Slice(displayList, func(i, j int) bool {
			return displayList[i].PeerID < displayList[j].PeerID
		})

		if len(displayList) > 0 && selectedIndex >= len(displayList) {
			selectedIndex = len(displayList) - 1
		}

		var uiContent string

		header := pterm.DefaultHeader.WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgMagenta)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Sprintf("📡 DECENTRALIZED MESH RADAR (Workers: %d | Raw Links: %d)", activeCount, peerCount)

		uiContent += header + "\n\n"

		if len(displayList) == 0 {
			uiContent += pterm.Warning.Sprintf("Listening to the Gossip grid... waiting for nodes to pulse.")
			area.Update(uiContent)
			return
		}

		tableData := pterm.TableData{{"", "PEER ID", "AUTHOR", "STATUS", "TASKS", "CORES", "CPU LOAD", "GPU VRAM", "RAM FREE", "LLM MODEL", "AGENT NAME"}}
		for i, w := range displayList {
			shortID := w.PeerID
			if len(shortID) > 10 {
				shortID = shortID[:6] + ".." + shortID[len(shortID)-4:]
			}

			statusDisplay := pterm.Green("🟢 AVAILABLE")
			if w.Status == "BUSY" || w.Status == "WORKING" {
				statusDisplay = pterm.Red("🔴 BUSY")
			}

			var taskDisplay string
			if w.MaxTasks > 0 {
				if w.CurrentTasks >= w.MaxTasks {
					taskDisplay = pterm.Red(fmt.Sprintf("%d/%d", w.CurrentTasks, w.MaxTasks))
				} else {
					taskDisplay = pterm.Cyan(fmt.Sprintf("%d/%d", w.CurrentTasks, w.MaxTasks))
				}
			} else {
				taskDisplay = pterm.Gray("-")
			}

			cpuBar := renderCPUBar(w.CPUUsagePct)

			gpuDisplay := pterm.Gray("No GPU")
			if w.HasGPU {
				if w.GPUUsagePct > 40.0 {
					gpuDisplay = pterm.Red(fmt.Sprintf("%.1f/%.1f GB", w.GPUUsedGB, w.GPUTotalGB))
				} else {
					gpuDisplay = pterm.Cyan(fmt.Sprintf("%.1f/%.1f GB", w.GPUUsedGB, w.GPUTotalGB))
				}
			}

			if i == selectedIndex {
				tableData = append(tableData, []string{
					pterm.LightGreen(" ▶ "), pterm.LightWhite(shortID), pterm.LightCyan(w.Author), statusDisplay, taskDisplay, pterm.LightWhite(strconv.Itoa(w.CPUCores)),
					cpuBar, gpuDisplay, pterm.LightWhite(fmt.Sprintf("%.1f GB", w.RAMFreeGB)),
					pterm.LightWhite(w.Model), pterm.LightWhite(w.AgentName),
				})
			} else {
				tableData = append(tableData, []string{
					"   ", pterm.Gray(shortID), pterm.Cyan(w.Author), statusDisplay, taskDisplay, pterm.Gray(strconv.Itoa(w.CPUCores)),
					cpuBar, gpuDisplay, pterm.Cyan(fmt.Sprintf("%.1f GB", w.RAMFreeGB)),
					pterm.Gray(w.Model), pterm.LightBlue(w.AgentName),
				})
			}
		}

		tableStr, _ := pterm.DefaultTable.WithHasHeader().WithHeaderRowSeparator("-").WithData(tableData).Srender()
		uiContent += tableStr + "\n"
		uiContent += pterm.LightWhite("⌨️  Use [UP/DOWN] arrows to select, [ENTER] to hire, [CTRL+C] to quit.")

		area.Update(uiContent)
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-uiCtx.Done():
				return
			case <-ticker.C:
				draw()
			}
		}
	}()

	draw()

	var selected types.WorkerProfile
	var confirmed bool

	keyboard.Listen(func(key keys.Key) (stop bool, err error) {
		if key.Code == keys.CtrlC {
			b.mu.Lock()
			fmt.Println("\nShutting down Boss node...")
			b.node.Host.Close()
			b.mu.Unlock()
			os.Exit(0)
		}

		// Use a local copy of length to prevent data races
		listLen := len(displayList)

		if listLen == 0 {
			return false, nil
		}

		switch key.Code {
		case keys.Up:
			selectedIndex = (selectedIndex - 1 + listLen) % listLen
			draw()
		case keys.Down:
			selectedIndex = (selectedIndex + 1) % listLen
			draw()
		case keys.Enter:
			if selectedIndex < len(displayList) {
				selected = displayList[selectedIndex]
				confirmed = true
			}
			return true, nil
		}
		return false, nil
	})
	return selected, confirmed
}

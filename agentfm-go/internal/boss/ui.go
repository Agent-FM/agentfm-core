package boss

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// selectWorkerInteractive blocks on keyboard input and returns
// (selected, confirmed, quit). `quit=true` means the user pressed Ctrl+C
// and Boss.Run should break out of its loop and shut down cleanly — we
// don't call os.Exit from the key callback because that skips every
// pending defer (cancelUI, area.Stop) and can leave the terminal in a
// bad state on some setups.
func (b *Boss) selectWorkerInteractive(parentCtx context.Context) (types.WorkerProfile, bool, bool) {
	uiCtx, cancelUI := context.WithCancel(parentCtx)
	defer cancelUI()

	// uiMu protects the closure-captured `displayList` and `selectedIndex`
	// against concurrent access from the redraw ticker goroutine and the
	// keyboard callback. Pre-fix, the keyboard callback's `len(displayList)`
	// raced with the ticker's `displayList = make(...)` reassignment,
	// failing -race the moment a TUI test exercised it.
	var (
		uiMu          sync.Mutex
		displayList   []types.WorkerProfile
		selectedIndex int
	)

	area, _ := pterm.DefaultArea.WithFullscreen(true).Start()
	defer area.Stop()

	draw := func() {
		// Pure read: pruning is handled by listenTelemetry's pruneTicker
		// (boss.go) so the TUI and /api/workers always agree on which
		// workers are visible. RLock is sufficient.
		b.mu.RLock()
		nextList := make([]types.WorkerProfile, 0, len(b.activeWorkers))
		for _, w := range b.activeWorkers {
			nextList = append(nextList, w)
		}
		activeCount := len(b.activeWorkers)
		peerCount := len(b.node.PubSub.ListPeers(network.TelemetryTopic))
		b.mu.RUnlock()

		sort.Slice(nextList, func(i, j int) bool {
			return nextList[i].PeerID < nextList[j].PeerID
		})

		uiMu.Lock()
		displayList = nextList
		if len(displayList) > 0 && selectedIndex >= len(displayList) {
			selectedIndex = len(displayList) - 1
		}
		// Snapshot for the rest of the render so we hold the lock for the
		// minimum window. The view we render is whatever was true at swap.
		view := displayList
		viewSelected := selectedIndex
		uiMu.Unlock()

		var uiContent string

		header := pterm.DefaultHeader.WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgMagenta)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Sprintf("📡 DECENTRALIZED MESH RADAR (Workers: %d | Raw Links: %d)", activeCount, peerCount)

		uiContent += header + "\n\n"

		if len(view) == 0 {
			uiContent += pterm.Warning.Sprintf("Listening to the Gossip grid... waiting for nodes to pulse.")
			area.Update(uiContent)
			return
		}

		tableData := pterm.TableData{{"", "PEER ID", "AUTHOR", "STATUS", "TASKS", "CORES", "CPU LOAD", "GPU VRAM", "RAM FREE", "LLM MODEL", "AGENT NAME"}}
		for i, w := range view {
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

			if i == viewSelected {
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
	var confirmed, quit bool

	keyboard.Listen(func(key keys.Key) (stop bool, err error) {
		if key.Code == keys.CtrlC {
			quit = true
			return true, nil
		}

		// Snapshot under uiMu so the redraw goroutine cannot reassign
		// displayList between len() and the indexed read below.
		uiMu.Lock()
		listLen := len(displayList)
		if listLen == 0 {
			uiMu.Unlock()
			return false, nil
		}
		switch key.Code {
		case keys.Up:
			selectedIndex = (selectedIndex - 1 + listLen) % listLen
			uiMu.Unlock()
			draw()
		case keys.Down:
			selectedIndex = (selectedIndex + 1) % listLen
			uiMu.Unlock()
			draw()
		case keys.Enter:
			if selectedIndex < listLen {
				selected = displayList[selectedIndex]
				confirmed = true
			}
			uiMu.Unlock()
			return true, nil
		default:
			uiMu.Unlock()
		}
		return false, nil
	})
	return selected, confirmed, quit
}

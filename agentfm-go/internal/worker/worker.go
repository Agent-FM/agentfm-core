package worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"agentfm/internal/network"

	"github.com/pterm/pterm"
)

type Config struct {
	ModelName          string
	AgentName          string
	AgentDesc          string
	ImageName          string
	AgentDir           string
	MaxConcurrentTasks int
	MaxCPU             float64 // dynamic CPU limit
	MaxGPU             float64 // dynamic GPU limit
	Author             string
}

type Worker struct {
	node         *network.MeshNode
	config       Config
	currentCPU   float64
	currentTasks int
	mu           sync.Mutex
}

func New(node *network.MeshNode, cfg Config) *Worker {
	return &Worker{node: node, config: cfg}
}

// RunLocalTest allows users to test their dockerfile/script locally without libp2p
func RunLocalTest(cfg Config, prompt string) error {
	w := &Worker{config: cfg}

	if err := w.buildSandboxImage(); err != nil {
		return err
	}

	fmt.Printf("\n🤖 Sending Prompt: '%s'\n", pterm.LightGreen(prompt))
	fmt.Println("--------------------------------------------------")

	// Use os.Stdout for testing locally
	outputDir := w.executePodman(prompt, os.Stdout, os.Stderr)

	fmt.Println("\n--------------------------------------------------")
	pterm.Success.Printfln("✅ Sandbox execution finished.\n📂 Artifacts saved to: %s", outputDir)

	return nil
}

func (w *Worker) Start(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("🚀 AGENTFM WORKER NODE ONLINE")

	if err := w.buildSandboxImage(); err != nil {
		pterm.Fatal.Printfln("Startup failed: %v", err)
		os.Exit(1)
	}

	w.printMetadata()
	go w.startTelemetry(ctx)

	w.node.Host.SetStreamHandler(network.TaskProtocol, w.handleTaskStream)
	w.node.Host.SetStreamHandler(network.FeedbackProtocol, w.handleFeedbackStream)

	w.waitForShutdown()
}

func (w *Worker) waitForShutdown() {
	fmt.Println()
	pterm.Info.Println("Worker is online and listening. Press CTRL+C to cleanly exit.")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	fmt.Println()
	pterm.Warning.Println("Received shutdown signal. Disconnecting from the mesh...")
	w.node.Host.Close()
	pterm.Success.Println("Safely offline. Goodbye!")
	os.Exit(0)
}

func (w *Worker) truncateWords(text string, maxWords int) string {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text
	}
	return strings.Join(words[:maxWords], " ") + "..."
}

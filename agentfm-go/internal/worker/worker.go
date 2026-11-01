package worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"agentfm/internal/network"

	netcore "github.com/libp2p/go-libp2p/core/network"
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

	// Limits are the per-task cgroup ceilings (roadmap R1). Distinct from
	// MaxCPU / MaxGPU above, which are *admission* thresholds — "is this
	// host too busy to accept another task" — and constrain nothing once a
	// container is running. Limits is what actually bounds a task.
	Limits ResourceLimits

	// Network is the task container's network namespace (roadmap S4),
	// resolved from -sandbox-network by ResolveNetworkMode. The zero value
	// is rejected by BuildRunArgs, so a caller that forgets to set it gets a
	// failed task rather than a permissive default.
	Network NetworkMode

	// IsWitness reports whether this worker process should advertise
	// the P2-2 witness role and (in P2-2) register the WitnessProtocol
	// stream handler. Set from the --witness flag on cmd/agentfm.
	// Plain workers default false; relay nodes default true.
	IsWitness bool

	// Capability is the operator-supplied capability tag (P3-1) —
	// kebab-case, e.g. "hr-specialist". Defaults to a kebabbed
	// version of AgentName when empty. Stored on every telemetry
	// envelope so Boss / probe coordinators can match agents.
	Capability string
}

type Worker struct {
	node         *network.MeshNode
	config       Config
	currentCPU   float64
	currentTasks int
	mu           sync.Mutex
	// wg tracks long-lived background goroutines (today: telemetry).
	// waitForShutdown waits on it before host.Close() so a publish
	// in flight cannot hit a torn-down libp2p host (race surfaced as
	// "use of closed connection" panics in pubsub send paths).
	wg sync.WaitGroup

	// P3-1: resolved once during Start. Empty when podman isn't on
	// the system or the image isn't present locally — worker still
	// runs but is "unattested" on the verification side.
	imageDigest string
	capability  string
}

func New(node *network.MeshNode, cfg Config) *Worker {
	return &Worker{node: node, config: cfg}
}

// printNetworkPosture tells the operator which network namespace task
// containers get. Called from both Worker.Start (long-lived worker) and
// RunLocalTest (single-shot test mode) so a workshop attendee running
// `agentfm -mode test` against a developer laptop sees the same posture
// the production worker would run with.
func printNetworkPosture(mode NetworkMode) {
	switch mode {
	case NetworkNone:
		pterm.Info.Println(
			"Containers run with --network none: the agent cannot reach this machine's " +
				"services, cloud metadata, or the internet. Agents that call a model over " +
				"HTTP (Ollama, vLLM) need -sandbox-network host.",
		)
	default:
		pterm.Warning.Println(
			"Containers run with --network host: the agent has full access to this " +
				"machine's network namespace, including loopback (127.0.0.1) services like " +
				"Ollama, internal admin endpoints, and cloud metadata (169.254.169.254). " +
				"The prompt driving the agent comes from whoever dispatched the task, so a " +
				"prompt-injected agent can be steered into any of these. Use a private " +
				"swarm (-swarmkey) unless every peer that can reach this worker is trusted.",
		)
	}
}

// RunLocalTest allows users to test their dockerfile/script locally without libp2p
func RunLocalTest(ctx context.Context, cfg Config, prompt string) error {
	w := &Worker{config: cfg}

	if err := w.buildSandboxImage(ctx); err != nil {
		return err
	}

	printNetworkPosture(w.config.Network)

	fmt.Printf("\n🤖 Sending Prompt: '%s'\n", pterm.LightGreen(prompt))
	fmt.Println("--------------------------------------------------")

	// Use os.Stdout for testing locally
	outputDir, execErr := w.executePodman(ctx, prompt, os.Stdout, os.Stderr)

	fmt.Println("\n--------------------------------------------------")

	// Surface the failure as a non-zero exit of `agentfm -mode test` rather
	// than printing a success banner. An operator validating an agent image
	// locally must not read "execution finished" when podman never ran.
	if execErr != nil {
		return fmt.Errorf("local sandbox run: %w", execErr)
	}

	pterm.Success.Printfln("✅ Sandbox execution finished.\n📂 Artifacts saved to: %s", outputDir)

	return nil
}

func (w *Worker) Start(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("🚀 AGENTFM WORKER NODE ONLINE")

	// Bind the root ctx to OS shutdown signals BEFORE the sandbox build so a
	// hung `podman build` (registry unreachable, broken Containerfile that
	// wedges a RUN step) is killable with Ctrl+C.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := w.buildSandboxImage(ctx); err != nil {
		pterm.Error.Printfln("Startup failed: %v", err)
		os.Exit(1)
	}

	// P3-1: resolve the OCI image digest now (post-build, so the
	// freshly-built image is present locally and inspectable). Cap
	// is just empty on failure — boss in strict-mode will reject.
	resolver := NewImageDigestResolver()
	if digest, err := resolver.ResolveDigest(ctx, w.config.ImageName); err == nil {
		w.imageDigest = digest
	} else {
		pterm.Warning.Printfln("⚠️  image digest resolve failed; running unattested: %v", err)
	}
	w.capability = w.config.Capability
	if w.capability == "" {
		w.capability = KebabCapability(w.config.AgentName)
	}

	printNetworkPosture(w.config.Network)

	// Surface the ceilings that will actually be applied. Without this an
	// operator has no way to confirm their -task-* flags took effect short of
	// inspecting a running container's cgroup — and a limit believed to be in
	// force but silently absent is worse than no limit at all.
	pterm.Info.Printfln("Per-task resource ceilings: %s", w.config.Limits.Describe())

	w.printMetadata()
	w.wg.Add(1)
	go w.startTelemetry(ctx)

	w.node.Host.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(ctx, s)
	})

	w.waitForShutdown(ctx)
}

func (w *Worker) waitForShutdown(ctx context.Context) {
	fmt.Println()
	pterm.Info.Println("Worker is online and listening. Press CTRL+C to cleanly exit.")

	<-ctx.Done()

	fmt.Println()
	pterm.Warning.Println("Received shutdown signal. Disconnecting from the mesh...")
	// Drain the telemetry goroutine BEFORE closing the host. Otherwise
	// an in-flight topic.Publish hits a torn-down libp2p host and
	// pubsub panics with "use of closed connection".
	w.wg.Wait()
	if err := w.node.Host.Close(); err != nil {
		pterm.Error.Printfln("Host close error: %v", err)
	}
	pterm.Success.Println("Safely offline. Goodbye!")
}

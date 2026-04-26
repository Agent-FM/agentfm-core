package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agentfm/internal/boss"
	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/worker"

	"github.com/pterm/pterm"
)

func main() {
	mode := flag.String("mode", "", "Node mode: 'boss', 'worker', 'relay', 'api', 'test', or 'genkey'")

	// Private Swarm & Network Flags
	swarmKey := flag.String("swarmkey", "", "Path to private swarm.key file (optional)")
	bootstrap := flag.String("bootstrap", "", "Custom bootstrap multiaddr (required for remote private swarms)")
	port := flag.Int("port", 0, "Listen port (0 for random. Relays should use 4001)")

	// API Gateway Port
	apiPort := flag.String("apiport", "8080", "Port for the local API gateway (only used in api mode)")

	// Observability: Prometheus /metrics listen address. Default is loopback
	// for safety; pass 0.0.0.0:<port> to expose to off-host scrapers.
	// Empty string disables the metrics server (worker and relay only;
	// boss-api always serves /metrics on its own port).
	promListen := flag.String("prom-listen", "", "Prometheus /metrics listen address (worker default 127.0.0.1:9090, relay default 127.0.0.1:9091, empty disables)")

	// Structured-logging controls. Format auto-detects: console on a TTY,
	// JSON otherwise. Operators running under systemd / docker / k8s want
	// JSON for log aggregation; humans on a laptop want console.
	logFormat := flag.String("log-format", obs.FormatAuto, "Log format: json, console, auto")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")

	// Test Mode Prompt
	testPrompt := flag.String("prompt", "", "Text prompt to send to the agent (used only in -mode test)")

	cfg := worker.Config{}
	flag.StringVar(&cfg.ModelName, "model", "", "The local LLM running")
	flag.StringVar(&cfg.AgentName, "agent", "", "The AI agent loaded")
	flag.StringVar(&cfg.AgentDesc, "desc", "", "Agent description")
	flag.StringVar(&cfg.ImageName, "image", "", "The Podman/Docker image to execute for this agent")
	flag.StringVar(&cfg.AgentDir, "agentdir", "", "Directory containing the agent code")
	flag.StringVar(&cfg.Author, "author", "Anonymous", "Name of the agent author/creator")
	// Worker capacity and thresholds
	flag.IntVar(&cfg.MaxConcurrentTasks, "maxtasks", 1, "Maximum concurrent tasks this worker can handle")
	flag.Float64Var(&cfg.MaxCPU, "maxcpu", 80.0, "Max CPU usage percentage before rejecting tasks")
	flag.Float64Var(&cfg.MaxGPU, "maxgpu", 80.0, "Max GPU VRAM usage percentage before rejecting tasks")

	setupHelpMenu()
	flag.Parse()

	// Handle Key Generation. pterm.Fatal already exits the process on
	// failure, so any os.Exit after a Fatal call would be unreachable.
	if *mode == "genkey" {
		if err := network.GenerateSwarmKey("swarm.key"); err != nil {
			pterm.Fatal.Printfln("❌ Failed to generate key: %v", err)
		}
		pterm.Success.Println("Generated private swarm key at ./swarm.key")
		pterm.Info.Println("Distribute this file to your VPS and trusted nodes to create a private mesh.")
		return
	}

	validateOperatorConfig(cfg)

	if *mode == "" {
		pterm.Error.Println("Please specify a mode: -mode boss, worker, relay, api, test, or genkey")
		os.Exit(1)
	}

	ctx := context.Background()

	netCfg := network.Config{
		Mode:         *mode,
		SwarmKeyPath: *swarmKey,
		ListenPort:   *port,
		BootstrapURL: *bootstrap,
	}

	// Set up the structured logger BEFORE any role-specific code runs so
	// every component-tagged log line has a consistent schema. The component
	// tag is the same as the mode for traceability.
	obs.Init(*mode, *logFormat, *logLevel)

	// Dispatch on mode. Each branch owns its own mesh setup and shutdown
	// so one mode can never leak a host into another.
	switch *mode {
	case "test":
		runTestMode(ctx, cfg, *testPrompt)
	case "relay":
		runRelayMode(ctx, netCfg, defaultPromListen(*promListen, "127.0.0.1:9091"))
	case "worker":
		runWorkerMode(ctx, netCfg, cfg, defaultPromListen(*promListen, "127.0.0.1:9090"))
	case "boss":
		runBossMode(ctx, netCfg)
	case "api":
		runAPIMode(ctx, netCfg, *apiPort)
	default:
		pterm.Error.Println("Invalid mode. Use 'boss', 'worker', 'relay', 'api', 'test', or 'genkey'.")
		os.Exit(1)
	}
}

// validateOperatorConfig bounds every operator-supplied limit up front.
// Numeric ranges mirror the help table. String caps match the
// WorkerProfile fields broadcast over GossipSub so a hostile author can
// not balloon a radar row on every other Boss in the mesh.
func validateOperatorConfig(cfg worker.Config) {
	if cfg.MaxCPU < 0 || cfg.MaxCPU > 99 {
		pterm.Fatal.Println("❌ Invalid config: -maxcpu must be between 0 and 99")
	}
	if cfg.MaxGPU < 0 || cfg.MaxGPU > 99 {
		pterm.Fatal.Println("❌ Invalid config: -maxgpu must be between 0 and 99")
	}
	if cfg.MaxConcurrentTasks < 1 || cfg.MaxConcurrentTasks > 1000 {
		pterm.Fatal.Println("❌ Invalid config: -maxtasks must be between 1 and 1000")
	}
	if len(cfg.AgentName) > 20 {
		pterm.Fatal.Printfln("❌ Invalid config: -agent name is too long (%d/20 chars max)", len(cfg.AgentName))
	}
	if len(cfg.ModelName) > 200 {
		pterm.Fatal.Printfln("❌ Invalid config: -model name is too long (%d/200 chars max)", len(cfg.ModelName))
	}
	if len(cfg.AgentDesc) > 3000 {
		pterm.Fatal.Printfln("❌ Invalid config: -desc is too long (%d/3000 chars max)", len(cfg.AgentDesc))
	}
	if len(cfg.Author) > 50 {
		pterm.Fatal.Printfln("❌ Invalid config: -author name is too long (%d/50 chars max)", len(cfg.Author))
	}
}

// runTestMode runs the local sandbox without any libp2p activity. This
// is the fastest way for an agent author to confirm their container
// image behaves before pushing it out to the mesh.
func runTestMode(ctx context.Context, cfg worker.Config, testPrompt string) {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgYellow)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("🧪 LOCAL SANDBOX TEST MODE")

	pterm.Info.Printfln("Testing Agent: %s", cfg.AgentName)
	pterm.Warning.Println("Bypassing P2P network. Executing container directly...")

	promptToUse := testPrompt
	if promptToUse == "" {
		fmt.Println()
		pterm.Info.Print("📝 Enter the prompt you want to send to your agent: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		promptToUse = strings.TrimSpace(input)

		if promptToUse == "" {
			pterm.Fatal.Println("❌ No prompt provided. Exiting test.")
		}
	}

	testCtx, stopTest := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopTest()
	if err := worker.RunLocalTest(testCtx, cfg, promptToUse); err != nil {
		pterm.Fatal.Printfln("❌ Local test failed: %v", err)
	}
}

// runRelayMode brings up a DHT-server mesh node without any worker or
// boss responsibilities. Useful for a developer who wants to run their
// own lighthouse on a spare VPS instead of using the public one.
func runRelayMode(ctx context.Context, netCfg network.Config, promListen string) {
	node, err := network.Setup(ctx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}

	relayCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if promListen != "" {
		go func() {
			if err := metrics.Serve(relayCtx, promListen); err != nil {
				pterm.Error.Printfln("metrics server: %v", err)
			}
		}()
		pterm.Success.Printfln("Metrics server: http://%s/metrics", promListen)
	}

	pterm.Success.Println("Relay Node Active. Press Ctrl+C to shut down.")
	pterm.Info.Println("📌 To connect nodes to this private swarm, start them with:")
	for _, addr := range node.Host.Addrs() {
		fmt.Printf("agentfm -mode boss -swarmkey ./swarm.key -bootstrap %s/p2p/%s\n", addr.String(), node.Host.ID().String())
	}

	<-relayCtx.Done()
	fmt.Println("\nShutting down relay...")
}

func runWorkerMode(ctx context.Context, netCfg network.Config, cfg worker.Config, promListen string) {
	node, err := network.Setup(ctx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}
	if promListen != "" {
		go func() {
			if err := metrics.Serve(ctx, promListen); err != nil {
				pterm.Error.Printfln("metrics server: %v", err)
			}
		}()
		pterm.Success.Printfln("Metrics server: http://%s/metrics", promListen)
	}
	w := worker.New(node, cfg)
	w.Start(ctx)
}

// defaultPromListen picks the listen address: if the operator passed a
// non-empty --prom-listen value it wins; otherwise fall back to the
// per-mode safe-by-default loopback address. To explicitly disable the
// metrics server, pass --prom-listen=- (a single dash).
func defaultPromListen(flagValue, modeDefault string) string {
	if flagValue == "-" {
		return ""
	}
	if flagValue != "" {
		return flagValue
	}
	return modeDefault
}

// runBossMode opens the interactive TUI for a human operator.
func runBossMode(ctx context.Context, netCfg network.Config) {
	// Bind SIGINT/SIGTERM to the root ctx so Ctrl+C unwinds cleanly when
	// the user is mid-task (NOT inside selectWorkerInteractive's
	// keyboard.Listen, which catches Ctrl+C separately). Without this the
	// Go runtime's default handler kills the process and skips every
	// defer: host.Close(), area.Stop(), open libp2p stream Reset.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	node, err := network.Setup(ctx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}
	b := boss.New(node)
	b.Run(ctx)
}

// runAPIMode starts the HTTP gateway that SDK clients talk to. The
// gateway's own error is returned all the way back here so the process
// exit code reflects whether the server came up cleanly.
func runAPIMode(ctx context.Context, netCfg network.Config, apiPort string) {
	node, err := network.Setup(ctx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}
	b := boss.New(node)
	if err := b.StartAPIServer(apiPort); err != nil {
		pterm.Fatal.Printfln("❌ API Gateway exited with error: %v", err)
	}
}

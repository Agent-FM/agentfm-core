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
	"agentfm/internal/network"
	"agentfm/internal/version"
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

	// Handle Key Generation
	if *mode == "genkey" {
		err := network.GenerateSwarmKey("swarm.key")
		if err != nil {
			pterm.Fatal.Printf("❌ Failed to generate key: %v\n", err)
		}
		pterm.Success.Println("Generated private swarm key at ./swarm.key")
		pterm.Info.Println("Distribute this file to your VPS and trusted nodes to create a private mesh.")
		os.Exit(0)
	}

	if cfg.MaxCPU < 0 || cfg.MaxCPU > 99 {
		pterm.Fatal.Println("❌ Invalid config: -maxcpu must be between 0 and 99")
		os.Exit(1)
	}
	if cfg.MaxGPU < 0 || cfg.MaxGPU > 99 {
		pterm.Fatal.Println("❌ Invalid config: -maxgpu must be between 0 and 99")
		os.Exit(1)
	}
	if cfg.MaxConcurrentTasks < 1 || cfg.MaxConcurrentTasks > 1000 {
		pterm.Fatal.Println("❌ Invalid config: -maxtasks must be between 1 and 1000")
		os.Exit(1)
	}
	if len(cfg.AgentName) > 20 {
		pterm.Fatal.Printfln("❌ Invalid config: -agent name is too long (%d/20 chars max)", len(cfg.AgentName))
		os.Exit(1)
	}
	if len(cfg.ModelName) > 200 {
		pterm.Fatal.Printfln("❌ Invalid config: -model name is too long (%d/40 chars max)", len(cfg.ModelName))
		os.Exit(1)
	}
	if len(cfg.AgentDesc) > 3000 {
		pterm.Fatal.Printfln("❌ Invalid config: -desc is too long (%d/1000 chars max)", len(cfg.AgentDesc))
		os.Exit(1)
	}
	if len(cfg.Author) > 50 {
		pterm.Fatal.Printfln("❌ Invalid config: -author name is too long (%d/50 chars max)", len(cfg.Author))
		os.Exit(1)
	}

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

	if *mode == "test" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgYellow)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Println("🧪 LOCAL SANDBOX TEST MODE")

		pterm.Info.Printfln("Testing Agent: %s", cfg.AgentName)
		pterm.Warning.Println("Bypassing P2P network. Executing container directly...")

		promptToUse := *testPrompt
		if promptToUse == "" {
			fmt.Println()
			pterm.Info.Print("📝 Enter the prompt you want to send to your agent: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			promptToUse = strings.TrimSpace(input)

			if promptToUse == "" {
				pterm.Fatal.Println("❌ No prompt provided. Exiting test.")
				os.Exit(1)
			}
		}

		testCtx, stopTest := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stopTest()
		err := worker.RunLocalTest(testCtx, cfg, promptToUse)
		if err != nil {
			pterm.Fatal.Printfln("❌ Local test failed: %v", err)
		}

		os.Exit(0)
	}

	if *mode == "relay" {
		node, err := network.Setup(ctx, netCfg)
		if err != nil {
			pterm.Fatal.Println(err)
		}

		pterm.Success.Println("Relay Node Active. Press Ctrl+C to shut down.")
		pterm.Info.Println("📌 To connect nodes to this private swarm, start them with:")
		for _, addr := range node.Host.Addrs() {
			fmt.Printf("agentfm -mode boss -swarmkey ./swarm.key -bootstrap %s/p2p/%s\n", addr.String(), node.Host.ID().String())
		}

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		fmt.Println("\nShutting down relay...")
		os.Exit(0)
	}

	// Handle Worker Mode
	if *mode == "worker" {
		node, err := network.Setup(ctx, netCfg)
		if err != nil {
			pterm.Fatal.Println(err)
		}
		w := worker.New(node, cfg)
		w.Start(ctx)

		// Handle Boss Mode (Interactive Terminal UI)
	} else if *mode == "boss" {
		node, err := network.Setup(ctx, netCfg)
		if err != nil {
			pterm.Fatal.Println(err)
		}
		b := boss.New(node)
		b.Run(ctx)

	} else if *mode == "api" {
		node, err := network.Setup(ctx, netCfg)
		if err != nil {
			pterm.Fatal.Println(err)
		}
		b := boss.New(node)
		if err := b.StartAPIServer(*apiPort); err != nil {
			pterm.Fatal.Printfln("❌ API Gateway exited with error: %v", err)
		}

	} else {
		pterm.Error.Println("Invalid mode. Use 'boss', 'worker', 'relay', 'api', 'test', or 'genkey'.")
		os.Exit(1)
	}
}

func setupHelpMenu() {
	flag.Usage = func() {
		fmt.Println()
		pterm.DefaultHeader.WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Printfln("🚀 AGENTFM CLI v%s", version.AppVersion)
		pterm.Info.Println("A global, peer-to-peer compute grid for containerized local AI.\n")

		pterm.DefaultSection.Println("Flags & Configuration")

		tableData := pterm.TableData{
			{"FLAG", "TYPE", "DESCRIPTION", "DEFAULT"},
			{pterm.Cyan("-mode"), pterm.LightMagenta("string"), "Node mode: 'boss', 'worker', 'relay', 'api', 'test', 'genkey'", pterm.Gray("none")},
			{pterm.Cyan("-prompt"), pterm.LightMagenta("string"), "Text prompt to send to agent (only for -mode test)", pterm.Gray("none")},
			{pterm.Cyan("-apiport"), pterm.LightMagenta("string"), "Port for the local API gateway", pterm.Gray("8080")},
			{pterm.Cyan("-swarmkey"), pterm.LightMagenta("string"), "Path to private swarm.key file", pterm.Gray("none")},
			{pterm.Cyan("-bootstrap"), pterm.LightMagenta("string"), "Custom relay/bootstrap multiaddr", pterm.Gray("public lighthouse")},
			{pterm.Cyan("-port"), pterm.LightMagenta("int"), "Network listen port", pterm.Gray("0 (Random)")},
			{pterm.Cyan("-agent"), pterm.LightMagenta("string"), "The AI agent loaded (max 20 chars)", pterm.Gray(`"HR Sick Leave Agent"`)},
			{pterm.Cyan("-agentdir"), pterm.LightMagenta("string"), "Directory containing the agent code", pterm.Gray(`"../agents/sick-leave"`)},
			{pterm.Cyan("-image"), pterm.LightMagenta("string"), "The Podman/Docker image tag to execute", pterm.Gray(`""`)},
			{pterm.Cyan("-desc"), pterm.LightMagenta("string"), "Short agent description (max 1000 chars)", pterm.Gray(`"Corporate Comms."`)},
			{pterm.Cyan("-model"), pterm.LightMagenta("string"), "Advertised core model capability (max 40 chars)", pterm.Gray(`"llama3.2"`)},
			{pterm.Cyan("-maxtasks"), pterm.LightMagenta("int"), "Max concurrent tasks this worker accepts (1-1000)", pterm.Gray(`"1"`)},
			{pterm.Cyan("-maxcpu"), pterm.LightMagenta("float"), "Max CPU usage % before rejecting tasks (0-99)", pterm.Gray(`"80.0"`)},
			{pterm.Cyan("-maxgpu"), pterm.LightMagenta("float"), "Max GPU VRAM usage % before rejecting tasks (0-99)", pterm.Gray(`"80.0"`)},
			{pterm.Cyan("-mode"), pterm.LightMagenta("string"), "Node mode: 'boss', 'worker', 'relay', 'api', 'test', 'genkey'", pterm.Gray("none")},
			{pterm.Cyan("-author"), pterm.LightMagenta("string"), "Name of the agent author/creator (max 50 chars)", pterm.Gray(`"Anonymous"`)},
		}

		pterm.DefaultTable.WithHasHeader().WithHeaderStyle(pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)).WithData(tableData).Render()
		fmt.Println()

		pterm.DefaultSection.Println("Examples & Use Cases")

		pterm.Println(pterm.Yellow("1. Test an Agent Locally (Interactive Prompt, Bypasses Network)"))
		pterm.Println(pterm.White("   ./agentfm -mode test \\"))
		pterm.Println(pterm.White("     -agentdir \"../agents/crewai/hr-specialist\" -image \"agentfm-hr:latest\" \\"))
		pterm.Println(pterm.White("     -model \"llama3.2\" -agent \"HR Specialist\" \\"))
		pterm.Println(pterm.White("     -desc \"Handles sick leave policies and corporate comms.\" -maxtasks 10\n"))

		pterm.Println(pterm.Yellow("2. Generate a Private Swarm Key (For closed enterprise darknets)"))
		pterm.Println(pterm.White("   ./agentfm -mode genkey\n"))

		pterm.Println(pterm.Yellow("3. Start a Boss Node (Interactive Terminal UI)"))
		pterm.Println(pterm.White("   ./agentfm -mode boss\n"))

		pterm.Println(pterm.Yellow("4. Start a Worker Node (Public Mesh, High Concurrency Text LLM)"))
		pterm.Println(pterm.White("   ./agentfm -mode worker \\"))
		pterm.Println(pterm.White("     -agentdir \"../agents/crewai/hr-specialist\" -image \"agentfm-hr:latest\" \\"))
		pterm.Println(pterm.White("     -model \"llama3.2\" -agent \"HR Specialist\" \\"))
		pterm.Println(pterm.White("     -desc \"Handles sick leave policies.\" -maxtasks 10 -maxcpu 90 -maxgpu 95\n"))

		pterm.Println(pterm.Yellow("5. Start a Private Darknet Worker (Requires Swarm Key & Relay Bootstrap)"))
		pterm.Println(pterm.White("   ./agentfm -mode worker \\"))
		pterm.Println(pterm.White("     -agentdir \"../agents/finance-analyzer\" -image \"agentfm-finance:internal\" \\"))
		pterm.Println(pterm.White("     -model \"mistral-nemo\" -agent \"Q3 Bot\" \\"))
		pterm.Println(pterm.White("     -desc \"Analyzes highly confidential CSV spreadsheets.\" \\"))
		pterm.Println(pterm.White("     -swarmkey \"./secrets/swarm.key\" \\"))
		pterm.Println(pterm.White("     -bootstrap \"/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW...\" -maxtasks 3\n"))

		pterm.Println(pterm.Yellow("6. Start a Dedicated Relay Node (VPS Lighthouse)"))
		pterm.Println(pterm.White("   ./agentfm -mode relay -port 4001\n"))
	}
}

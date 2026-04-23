package main

import (
	"flag"
	"fmt"

	"agentfm/internal/version"

	"github.com/pterm/pterm"
)

// setupHelpMenu replaces flag.Usage with a branded pterm-rendered table
// and a set of worked examples. We install it before flag.Parse so that
// "agentfm -h" and "agentfm --help" both land on the custom screen.
func setupHelpMenu() {
	flag.Usage = func() {
		fmt.Println()
		pterm.DefaultHeader.WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Printfln("🚀 AGENTFM CLI v%s", version.AppVersion)
		pterm.Info.Println("A global, peer-to-peer compute grid for containerized local AI.")
		fmt.Println()

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
			{pterm.Cyan("-desc"), pterm.LightMagenta("string"), "Short agent description (max 3000 chars)", pterm.Gray(`"Corporate Comms."`)},
			{pterm.Cyan("-model"), pterm.LightMagenta("string"), "Advertised core model capability (max 200 chars)", pterm.Gray(`"llama3.2"`)},
			{pterm.Cyan("-maxtasks"), pterm.LightMagenta("int"), "Max concurrent tasks this worker accepts (1-1000)", pterm.Gray(`"1"`)},
			{pterm.Cyan("-maxcpu"), pterm.LightMagenta("float"), "Max CPU usage % before rejecting tasks (0-99)", pterm.Gray(`"80.0"`)},
			{pterm.Cyan("-maxgpu"), pterm.LightMagenta("float"), "Max GPU VRAM usage % before rejecting tasks (0-99)", pterm.Gray(`"80.0"`)},
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

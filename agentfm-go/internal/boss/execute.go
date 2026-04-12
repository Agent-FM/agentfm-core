package boss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pterm/pterm"
)

func (b *Boss) executeFlow(worker types.WorkerProfile) {
	fmt.Print("\033[H\033[2J")

	boxContent := pterm.LightMagenta("Name: ") + pterm.White(worker.AgentName) + "\n" +
		pterm.LightMagenta("Capabilities: ") + pterm.White(worker.AgentDesc) + "\n" +
		pterm.LightMagenta("Model: ") + pterm.White(worker.Model)

	pterm.DefaultBox.WithTitle(pterm.LightGreen("🕵️ AGENT SECURED")).WithTitleTopLeft().Println(boxContent)
	fmt.Println()

	prompt, _ := pterm.DefaultInteractiveTextInput.Show("📝 Enter task prompt (or type 'back' to return to radar)")
	prompt = strings.TrimSpace(prompt)

	if strings.ToLower(prompt) == "back" || prompt == "" {
		return
	}

	targetPeerID, _ := peer.Decode(worker.PeerID)
	fmt.Println()
	pterm.Info.Printfln("Initiating secure encrypted TCP tunnel to %s...", pterm.Cyan(targetPeerID.String()[:8]))

	s := b.dialOmni(targetPeerID)
	if s == nil {
		return
	}

	taskJSON := fmt.Sprintf(`{"version": "%s", "task": "agent_task", "data": "%s"}`, version.AppVersion, prompt)
	s.Write([]byte(taskJSON))
	s.CloseWrite()

	pterm.DefaultSection.WithLevel(2).Println("🤖 LIVE AGENT STREAM")

	deadman := &timeoutReader{stream: s, timeout: 10 * time.Minute}
	if _, err := io.Copy(os.Stdout, deadman); err != nil {
		if os.IsTimeout(err) {
			pterm.Error.Println("\n⏳ WORKER GHOSTED: Stream timed out after 10 minutes.")
		} else {
			pterm.Error.Printfln("\n💥 Stream broken: %v", err)
		}
	}

	fmt.Println()
	pterm.DefaultSection.WithLevel(2).Println("END OF STREAM")
	pterm.Success.Println("Tunnel safely closed.")

	fmt.Println()
	pterm.DefaultInteractiveContinue.
		WithDefaultText(pterm.LightWhite("Task execution completed. Press [ENTER] to continue to the feedback menu")).
		Show()

	b.handleFeedbackLoop(targetPeerID)
}

func (b *Boss) dialOmni(target peer.ID) netcore.Stream {
	var addrs []multiaddr.Multiaddr

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Punching NAT to reach %s...", target.String()[:8]))

	if peerInfo := b.node.Host.Peerstore().PeerInfo(target); len(peerInfo.Addrs) > 0 {
		spinner.Success("Peer found in local cache. Bypassing DHT.")
		addrs = append(addrs, peerInfo.Addrs...)
	} else {
		spinner.UpdateText("Node not in cache. Querying global DHT...")
		lookupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if info, err := b.node.DHT.FindPeer(lookupCtx, target); err == nil {
			addrs = append(addrs, info.Addrs...)
		} else {
			spinner.Fail("DHT connection failed.")
			time.Sleep(2 * time.Second)
			return nil
		}
	}

	if relayMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p-circuit/p2p/%s", b.node.RelayAddr, target.String())); err == nil {
		addrs = append(addrs, relayMA)
	}

	b.node.Host.Peerstore().SetAddrs(target, addrs, 2*time.Minute)

	spinner.UpdateText("Dialing peer directly and via Hetzner Relay simultaneously...")

	s, err := b.node.Host.NewStream(context.Background(), target, network.TaskProtocol)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to connect via Direct IP or Relay: %v", err))
		time.Sleep(3 * time.Second)
		return nil
	}

	spinner.Success("P2P Tunnel Established! Secure encrypted stream active.")
	return s
}

func (b *Boss) handleFeedbackLoop(target peer.ID) {
	fmt.Println()
	leave, _ := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show("📝 Leave feedback for the node operator?")
	if leave {
		feedback, _ := pterm.DefaultInteractiveTextInput.Show("Type your feedback")
		if strings.TrimSpace(feedback) != "" {
			pterm.Info.Println("Opening secure feedback tunnel...")

			if fs, err := b.node.Host.NewStream(context.Background(), target, network.FeedbackProtocol); err == nil {
				payload := map[string]string{"feedback": feedback, "timestamp": time.Now().Format(time.RFC3339)}
				json.NewEncoder(fs).Encode(payload)
				fs.Close()
				pterm.Success.Println("Feedback delivered directly to the worker! 💌")
			} else {
				pterm.Error.Printfln("Failed to deliver feedback: %v", err)
			}
		}
	}
}

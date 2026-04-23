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

func (b *Boss) executeFlow(ctx context.Context, worker types.WorkerProfile) {
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

	targetPeerID, err := peer.Decode(worker.PeerID)
	if err != nil {
		pterm.Error.Printfln("Invalid worker peer ID %q: %v", worker.PeerID, err)
		return
	}
	fmt.Println()
	pterm.Info.Printfln("Initiating secure encrypted TCP tunnel to %s...", pterm.Cyan(targetPeerID.String()[:8]))

	s := b.dialOmni(ctx, targetPeerID)
	if s == nil {
		return
	}

	streamSuccess := false
	defer func() {
		if streamSuccess {
			_ = s.Close()
		} else {
			_ = s.Reset()
		}
	}()

	if err := s.SetWriteDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
		pterm.Error.Printfln("Failed to set write deadline: %v", err)
		return
	}

	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    prompt,
	}
	if err := json.NewEncoder(s).Encode(&payload); err != nil {
		pterm.Error.Printfln("Failed to send prompt: %v", err)
		return
	}
	if err := s.CloseWrite(); err != nil {
		pterm.Error.Printfln("Failed to half-close tunnel: %v", err)
		return
	}

	pterm.DefaultSection.WithLevel(2).Println("🤖 LIVE AGENT STREAM")

	deadman := &timeoutReader{stream: s, timeout: network.TaskExecutionTimeout}
	if _, err := io.Copy(os.Stdout, deadman); err != nil {
		if os.IsTimeout(err) {
			pterm.Error.Println("\n⏳ WORKER GHOSTED: Stream timed out.")
		} else {
			pterm.Error.Printfln("\n💥 Stream broken: %v", err)
		}
		return
	}

	streamSuccess = true

	fmt.Println()
	pterm.DefaultSection.WithLevel(2).Println("END OF STREAM")
	pterm.Success.Println("Tunnel safely closed.")

	fmt.Println()
	pterm.DefaultInteractiveContinue.
		WithDefaultText(pterm.LightWhite("Task execution completed. Press [ENTER] to continue to the feedback menu")).
		Show()

	b.handleFeedbackLoop(ctx, targetPeerID)
}

// dialOmni tries to open a task stream to the target peer. It first
// looks in the libp2p peerstore (zero-RTT if we've seen this peer
// recently), then falls back to a DHT lookup and finally pins the
// circuit-relay address as a backup route. Every context we derive
// inherits from the caller's ctx so a Ctrl+C mid-dial actually unwinds.
func (b *Boss) dialOmni(ctx context.Context, target peer.ID) netcore.Stream {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Punching NAT to reach %s...", target.String()[:8]))
	s, err := b.dialWorkerStream(ctx, target)
	if err != nil {
		spinner.Fail(err.Error())
		time.Sleep(2 * time.Second)
		return nil
	}
	spinner.Success("P2P Tunnel Established! Secure encrypted stream active.")
	return s
}

// dialWorkerStream is the pterm-free core of dialOmni. HTTP-path callers
// (OpenAI handlers, future server-side dials) use this directly so they
// don't drag a TUI spinner — and its known concurrent-state race — into
// goroutine-rich code paths.
func (b *Boss) dialWorkerStream(ctx context.Context, target peer.ID) (netcore.Stream, error) {
	var addrs []multiaddr.Multiaddr

	if peerInfo := b.node.Host.Peerstore().PeerInfo(target); len(peerInfo.Addrs) > 0 {
		addrs = append(addrs, peerInfo.Addrs...)
	} else {
		if b.node.DHT == nil {
			return nil, fmt.Errorf("peer %s not in cache and DHT unavailable", target.String()[:8])
		}
		lookupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		info, err := b.node.DHT.FindPeer(lookupCtx, target)
		if err != nil {
			return nil, fmt.Errorf("DHT lookup for %s failed: %w", target.String()[:8], err)
		}
		addrs = append(addrs, info.Addrs...)
	}

	if relayMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p-circuit/p2p/%s", b.node.RelayAddr, target.String())); err == nil {
		addrs = append(addrs, relayMA)
	}

	b.node.Host.Peerstore().SetAddrs(target, addrs, 2*time.Minute)

	dialCtx, cancel := context.WithTimeout(ctx, network.StreamDialTimeout)
	defer cancel()
	s, err := b.node.Host.NewStream(dialCtx, target, network.TaskProtocol)
	if err != nil {
		return nil, fmt.Errorf("dial via direct or relay failed: %w", err)
	}
	return s, nil
}

func (b *Boss) handleFeedbackLoop(ctx context.Context, target peer.ID) {
	fmt.Println()
	leave, _ := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show("📝 Leave feedback for the node operator?")
	if !leave {
		return
	}
	feedback, _ := pterm.DefaultInteractiveTextInput.Show("Type your feedback")
	if strings.TrimSpace(feedback) == "" {
		return
	}

	pterm.Info.Println("Opening secure feedback tunnel...")

	dialCtx, cancel := context.WithTimeout(ctx, network.StreamDialTimeout)
	defer cancel()

	fs, err := b.node.Host.NewStream(dialCtx, target, network.FeedbackProtocol)
	if err != nil {
		pterm.Error.Printfln("Failed to deliver feedback: %v", err)
		return
	}

	reset := true
	defer func() {
		if reset {
			_ = fs.Reset()
		} else {
			_ = fs.Close()
		}
	}()

	if err := fs.SetWriteDeadline(time.Now().Add(network.FeedbackStreamTimeout)); err != nil {
		pterm.Error.Printfln("Failed to set feedback deadline: %v", err)
		return
	}

	payload := map[string]string{"feedback": feedback, "timestamp": time.Now().Format(time.RFC3339)}
	if err := json.NewEncoder(fs).Encode(payload); err != nil {
		pterm.Error.Printfln("Failed to deliver feedback: %v", err)
		return
	}

	reset = false
	pterm.Success.Println("Feedback delivered directly to the worker! 💌")
}

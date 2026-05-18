package boss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/types"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pterm/pterm"
)

// menuPickerForTest, when non-nil, overrides the pterm interactive select in
// showPeerMenu. Tests inject a deterministic picker via SetMenuPickerForTest.
// peerViewHookForTest, when non-nil, overrides the real viewPeerHistory call
// in executeFlow. Tests inject a hook via SetPeerViewHookForTest.
//
// Both are unexported fields intentionally — test code reaches them via the
// exported Set* methods; production code never touches them.

func (b *Boss) executeFlow(ctx context.Context, worker types.WorkerProfile) {
	for {
		choice, err := b.showPeerMenu(worker)
		if err != nil || choice == "Back to radar" {
			return
		}
		switch choice {
		case "Execute task":
			b.executeTaskFlow(ctx, worker)
			return
		case "View ratings & feedback":
			if b.peerViewHookForTest != nil {
				b.peerViewHookForTest(ctx, worker.PeerID)
			} else {
				b.viewPeerHistory(ctx, worker.PeerID)
			}
			// Loop back to the menu after viewing history.
		}
	}
}

// showPeerMenu renders the agent-info box and an interactive three-option
// menu. Returns the chosen option string or an error. Uses
// menuPickerForTest if set (for unit tests), otherwise falls back to pterm.
func (b *Boss) showPeerMenu(worker types.WorkerProfile) (string, error) {
	options := []string{"Execute task", "View ratings & feedback", "Back to radar"}

	if b.menuPickerForTest != nil {
		return b.menuPickerForTest(options)
	}

	fmt.Print("\033[H\033[2J")

	honesty := 0.0
	if b.reputationEngine != nil {
		honesty = b.reputationEngine.Score(worker.PeerID)
	}
	dispatchBadge := pterm.Green("✓ allowed")
	floor := b.reputationFloor
	if floor == 0 {
		floor = -1.0 // safe default until Phase 8
	}
	if honesty < floor {
		dispatchBadge = pterm.Red(fmt.Sprintf("✗ rejected (below floor %.2f)", floor))
	}

	header := pterm.LightMagenta("Name: ") + pterm.White(worker.AgentName) + "\n" +
		pterm.LightMagenta("Model: ") + pterm.White(worker.Model) + "\n" +
		pterm.LightMagenta("Image: ") + pterm.White(shortDigest(worker.AgentImageDigest)) + "\n" +
		pterm.LightMagenta("Honesty: ") + pterm.White(fmt.Sprintf("%+.2f", honesty)) + "\n" +
		pterm.LightMagenta("Dispatch: ") + dispatchBadge

	pterm.DefaultBox.WithTitle(pterm.LightGreen("🕵️  AGENT SELECTED")).WithTitleTopLeft().Println(header)
	fmt.Println()

	return pterm.DefaultInteractiveSelect.
		WithOptions(options).
		Show()
}

// shortDigest abbreviates a digest like "sha256:abc12345..." to 16 chars + "...".
func shortDigest(d string) string {
	if len(d) <= 16 {
		return d
	}
	return d[:16] + "..."
}

// executeTaskFlow contains the original executeFlow body: prompt → dispatch →
// stream → handleFeedbackLoop. Factored out so executeFlow can branch between
// task execution and peer-history viewing without duplicating the task path.
func (b *Boss) executeTaskFlow(ctx context.Context, worker types.WorkerProfile) {
	prompt, _ := pterm.DefaultInteractiveTextInput.Show("📝 Enter task prompt (or type 'back' to return to radar)")
	prompt = strings.TrimSpace(prompt)

	if strings.ToLower(prompt) == "back" || prompt == "" {
		return
	}

	// Per-task observability. The deferred closure runs once at function
	// return; status mutates as the handler progresses and whatever value
	// it holds at exit is what gets reported.
	started := time.Now()
	status := metrics.StatusError
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(started).Seconds())
		metrics.TasksTotal.WithLabelValues(status).Inc()
	}()

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

	taskID := newCompletionID("task_")
	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    prompt,
		TaskID:  taskID,
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
			status = metrics.StatusTimeout
			pterm.Error.Println("\n⏳ WORKER GHOSTED: Stream timed out.")
		} else {
			pterm.Error.Printfln("\n💥 Stream broken: %v", err)
		}
		return
	}

	streamSuccess = true
	status = metrics.StatusOK

	fmt.Println()
	pterm.DefaultSection.WithLevel(2).Println("END OF STREAM")
	pterm.Success.Println("Tunnel safely closed.")

	fmt.Println()
	pterm.DefaultInteractiveContinue.
		WithDefaultText(pterm.LightWhite("Task execution completed. Press [ENTER] to continue to the feedback menu")).
		Show()

	b.handleFeedbackLoop(ctx, targetPeerID, taskID)
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
		// Brief pause so the operator reads the failure, but bail early
		// if the user has already hit Ctrl+C or the parent ctx fired.
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}
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
	// Fast path: we already have a live libp2p connection to this peer.
	// NewStream multiplexes onto it without needing peerstore addrs or
	// a DHT lookup. Required when the worker dialed the boss first
	// (the common shape in NAT'd public-mesh deployments — workers
	// punch out to the lighthouse, bosses see them inbound). In that
	// case the boss's peerstore knows only the worker's ephemeral
	// source-port from the inbound connection, which isn't dialable,
	// but the underlying tunnel is fully usable.
	if b.node.Host.Network().Connectedness(target) == netcore.Connected {
		dialCtx, cancel := context.WithTimeout(ctx, network.StreamDialTimeout)
		s, err := b.node.Host.NewStream(dialCtx, target, network.TaskProtocol)
		cancel()
		if err == nil {
			return s, nil
		}
		// Connection went away mid-call (rare). Fall through to the
		// peerstore / DHT address-book path so a fresh dial can be
		// attempted.
	}

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

	// Only attempt the circuit-relay path when the relay is actually
	// reachable. A dead relay would otherwise add a guaranteed-fail
	// dial to every task, costing StreamDialTimeout per request.
	if b.node.RelayPeerID != "" &&
		b.node.Host.Network().Connectedness(b.node.RelayPeerID) == netcore.Connected {
		if relayMA, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p-circuit/p2p/%s", b.node.RelayAddr, target.String())); err == nil {
			addrs = append(addrs, relayMA)
		}
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

func (b *Boss) handleFeedbackLoop(ctx context.Context, target peer.ID, taskID string) {
	fmt.Println()
	leave, _ := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show("📝 Leave feedback for the node operator?")
	if !leave {
		return
	}

	text, _ := pterm.DefaultInteractiveTextInput.Show("Type your feedback")
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	ratingStr, _ := pterm.DefaultInteractiveTextInput.Show("Leave a numeric rating too? [-1.0 to +1.0, blank to skip]")
	ratingStr = strings.TrimSpace(ratingStr)

	var parsedRating *float64
	if ratingStr != "" {
		v, err := strconv.ParseFloat(ratingStr, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < -1.0 || v > 1.0 {
			pterm.Warning.Println("Invalid rating value — saving comment without a rating.")
		} else {
			parsedRating = &v
		}
	}

	if err := b.appendFeedbackComment(ctx, target, taskID, text, parsedRating); err != nil {
		pterm.Error.Printfln("Failed to persist feedback: %v", err)
		return
	}
	pterm.Success.Println("Feedback signed, persisted, and gossipped to the mesh. 💌")
}

// appendFeedbackComment writes a signed Comment to the ledger and, if
// optionalRatingScore is non-nil, also writes a linked Rating with
// dimension "honesty". Both are auto-gossiped via the ledger's subscribe
// loop. Returns the underlying ledger error on failure; the caller is
// responsible for user-facing reporting.
func (b *Boss) appendFeedbackComment(ctx context.Context, target peer.ID, taskID, text string, optionalRatingScore *float64) error {
	if b.ledger == nil {
		return errors.New("ledger disabled")
	}
	if b.commentsStore == nil {
		return errors.New("comments store disabled")
	}

	cid, err := b.commentsStore.Put([]byte(text))
	if err != nil {
		return fmt.Errorf("comments store: %w", err)
	}

	now := time.Now().UnixNano()
	myPID := []byte(b.node.Host.ID())
	subjectBytes := []byte(target)

	commentEntry := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: &pb.Comment{
		RaterPeerId:     myPID,
		SubjectPeerId:   subjectBytes,
		TextCid:         cid,
		Language:        "en",
		TimestampUnixNs: now,
	}}}
	if _, err := b.ledger.Append(ctx, commentEntry); err != nil {
		return fmt.Errorf("append comment: %w", err)
	}

	if optionalRatingScore != nil {
		ratingEntry := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     myPID,
			SubjectPeerId:   subjectBytes,
			Dimension:       "honesty",
			Score:           *optionalRatingScore,
			Context:         "interactive",
			TimestampUnixNs: now + 1,
		}}}
		if _, err := b.ledger.Append(ctx, ratingEntry); err != nil {
			return fmt.Errorf("append rating: %w", err)
		}
	}
	return nil
}


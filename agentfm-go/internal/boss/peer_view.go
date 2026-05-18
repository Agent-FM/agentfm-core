// peer_view.go implements GatherPeerEntries, a shared helper that collects
// ledger entries (both own-log and inbox) for a given subject peer,
// sorted newest-first, and capped at limit. Used by both:
//   - GET /v1/peers/{id}/log (HTTP API, sub-task 1.3)
//   - GET /v1/peers/{id}     (single-peer summary, sub-task 1.4)
//
// Also implements KnownPeer / ListKnownPeers (Phase 6 offline-peer
// visibility, sub-task 6.2): combines in-memory activeWorkers (online
// peers from telemetry) with store.DistinctSubjects (peers known only
// via ledger entries) into one sorted list.
package boss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
	"google.golang.org/protobuf/proto"
)

// PeerEntry is one decoded ledger entry about a subject peer.
type PeerEntry struct {
	ReceivedAt        time.Time `json:"received_at"`
	Kind              string    `json:"kind"` // "Rating" | "Comment"
	Rater             peer.ID   `json:"rater_peer_id"`
	Dimension         string    `json:"dimension,omitempty"`
	Score             float64   `json:"score,omitempty"`
	Context           string    `json:"context,omitempty"`
	Language          string    `json:"language,omitempty"`
	TextCID           []byte    `json:"text_cid,omitempty"`
	RaterStatus       string    `json:"rater_status"`
	RaterHonestyScore float64   `json:"rater_honesty_score"`
}

// GatherPeerEntries walks both IterateAllOwnEntries and
// IterateAllInboxEntries, decodes each SignedEntry proto, filters to those
// whose SubjectPeerId matches subject, returns newest-first sorted, capped
// at limit. RaterStatus and RaterHonestyScore are left zero — callers that
// need them should decorate after calling (see handlePeerLog).
//
// This is a lifted/refactored version of the CLI's gatherReputationView in
// cmd/agentfm/reputation.go. Both can co-exist — the CLI version returns a
// richer reputationView struct; this one returns []PeerEntry for HTTP use.
func GatherPeerEntries(ctx context.Context, s *store.Store, subject peer.ID, limit int) ([]PeerEntry, error) {
	subjectBytes := []byte(subject)

	var entries []PeerEntry

	collect := func(payload []byte, receivedAtNs int64) {
		var signed pb.SignedEntry
		if err := proto.Unmarshal(payload, &signed); err != nil {
			return
		}
		receivedAt := time.Unix(0, receivedAtNs)
		switch body := signed.GetBody().(type) {
		case *pb.SignedEntry_Rating:
			r := body.Rating
			if r == nil {
				return
			}
			if !bytesEqualPB(r.SubjectPeerId, subjectBytes) {
				return
			}
			entries = append(entries, PeerEntry{
				ReceivedAt: receivedAt,
				Kind:       "Rating",
				Rater:      peer.ID(r.RaterPeerId),
				Dimension:  r.Dimension,
				Score:      r.Score,
				Context:    r.Context,
			})
		case *pb.SignedEntry_Comment:
			c := body.Comment
			if c == nil {
				return
			}
			if !bytesEqualPB(c.SubjectPeerId, subjectBytes) {
				return
			}
			entries = append(entries, PeerEntry{
				ReceivedAt: receivedAt,
				Kind:       "Comment",
				Rater:      peer.ID(c.RaterPeerId),
				Language:   c.Language,
				TextCID:    c.TextCid,
			})
		}
	}

	if err := s.IterateAllOwnEntries(ctx, func(e *store.Entry) error {
		collect(e.Payload, e.InsertedAt)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.IterateAllInboxEntries(ctx, func(e *store.InboxEntry) error {
		collect(e.Payload, e.ReceivedAt)
		return nil
	}); err != nil {
		return nil, err
	}

	// Sort newest-first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ReceivedAt.After(entries[j].ReceivedAt)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// bytesEqualPB compares two byte slices for equality.
func bytesEqualPB(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Phase 7: TUI peer-history view
// ---------------------------------------------------------------------------

// renderPeerView writes the peer-history view to out. Used by both the TUI
// (out=os.Stdout) and tests (out=bytes.Buffer). Keeps the rendering logic in
// one place.
func (b *Boss) renderPeerView(out io.Writer, ctx context.Context, peerIDStr string) {
	subjectPID, err := peer.Decode(peerIDStr)
	if err != nil {
		fmt.Fprintf(out, "Invalid peer ID %q: %v\n", peerIDStr, err)
		return
	}

	shortPeer := shortID(peerIDStr, 12)
	fmt.Fprintf(out, "📜 PEER HISTORY · %s\n\n", shortPeer)

	// --- Summary box -------------------------------------------------------
	agentName := "(unknown)"
	statusStr := "offline"
	isEquiv := false
	honesty := 0.0
	floor := b.reputationFloor
	if floor == 0 {
		floor = -1.0
	}

	b.mu.RLock()
	if p, ok := b.activeWorkers[peerIDStr]; ok {
		agentName = nonEmpty(p.AgentName, "(unknown)")
		statusStr = pterm.Green("✓ online")
	}
	b.mu.RUnlock()

	if b.reputationEngine != nil {
		honesty = b.reputationEngine.Score(peerIDStr)
	}
	if b.ledger != nil {
		isEquiv, _ = b.ledger.IsEquivocator(ctx, []byte(subjectPID))
	}

	var entriesList []PeerEntry
	if b.readStore != nil {
		entriesList, _ = GatherPeerEntries(ctx, b.readStore, subjectPID, 0)
	}

	// Determine last-seen age for offline status.
	b.mu.RLock()
	ls := b.lastSeen[peerIDStr]
	b.mu.RUnlock()
	if statusStr == "offline" && !ls.IsZero() {
		statusStr = "offline " + compactAge(time.Since(ls))
	}

	honestyStr := formatScore(honesty, floor)
	equivStr := formatEquiv(isEquiv)

	fmt.Fprintf(out, "Agent: %s\n", agentName)
	fmt.Fprintf(out, "Status: %s\n", statusStr)
	fmt.Fprintf(out, "Honesty: %s\n", honestyStr)
	fmt.Fprintf(out, "Entries: %d\n", len(entriesList))
	fmt.Fprintf(out, "Equivocator: %s\n\n", equivStr)

	if len(entriesList) == 0 {
		fmt.Fprintf(out, "No ledger entries about this peer yet.\n")
		return
	}

	// --- Entry table -------------------------------------------------------
	// Build plain-text table: WHEN | KIND | RATER | DETAIL
	fmt.Fprintf(out, "%-6s  %-7s  %-25s  %s\n", "WHEN", "KIND", "RATER", "DETAIL")
	fmt.Fprintf(out, "%s\n", "------  -------  -------------------------  ------")

	for _, e := range entriesList {
		when := compactAge(time.Since(e.ReceivedAt))
		raterStr := shortID(e.Rater.String(), 12)

		// Mark rater as [unverified] when their honesty score is below 0.1.
		// When there is no reputation engine, the score is effectively 0.0
		// (no trust data), so all raters are unverified by default.
		raterScore := 0.0
		if b.reputationEngine != nil {
			raterScore = b.reputationEngine.Score(e.Rater.String())
		}
		unverified := raterScore < 0.1
		if unverified {
			raterStr = "[unverified] " + raterStr
		}

		var detail string
		switch e.Kind {
		case "Rating":
			ctx := e.Context
			if ctx == "" {
				ctx = "—"
			}
			detail = fmt.Sprintf("%+.2f %s · %s", e.Score, e.Dimension, ctx)
		case "Comment":
			lang := nonEmpty(e.Language, "?")
			body := "(unavailable)"
			if b.commentsStore != nil {
				if text, err := b.commentsStore.Get(e.TextCID); err != nil {
					body = "(missing body)"
				} else {
					body = truncateStr(string(text), 60)
				}
			}
			detail = fmt.Sprintf("[%s] %s", lang, body)
		}

		fmt.Fprintf(out, "%-6s  %-7s  %-25s  %s\n", when, e.Kind, raterStr, detail)
	}
}

// viewPeerHistory renders the peer-view screen to stdout via fmt and pterm.
// Blocks on a "Press [ENTER] to return" prompt at the end.
func (b *Boss) viewPeerHistory(ctx context.Context, peerIDStr string) {
	fmt.Print("\033[H\033[2J")
	b.renderPeerView(os.Stdout, ctx, peerIDStr)
	fmt.Println()
	pterm.DefaultInteractiveContinue.WithDefaultText("Press [ENTER] to return").Show()
}

// RenderPeerView returns the rendered peer-view as a string. Used by
// integration tests and TestTrustEndToEnd to assert on rendered output.
func (b *Boss) RenderPeerView(ctx context.Context, peerIDStr string) string {
	var buf bytes.Buffer
	b.renderPeerView(&buf, ctx, peerIDStr)
	return buf.String()
}

// ---------------------------------------------------------------------------
// Small formatting helpers
// ---------------------------------------------------------------------------

// compactAge returns a human-readable short duration string:
// "30s", "5m", "2h", "3d".
func compactAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// truncateStr returns s[:n]+"..." if len(s) > n, else s.
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// nonEmpty returns s if non-empty, else fallback.
func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// formatScore returns a color-coded honesty score string.
func formatScore(s, floor float64) string {
	str := fmt.Sprintf("%+.2f", s)
	if s <= floor {
		return pterm.Red(str)
	}
	if s >= 0.5 {
		return pterm.Green(str)
	}
	return str
}

// formatEquiv returns a short equivocator indicator.
func formatEquiv(isEquiv bool) string {
	if isEquiv {
		return pterm.Red("⚠ YES — permanently floored at -1.00")
	}
	return "no"
}

// KnownPeer is the operator-facing view of a peer the boss has heard about,
// whether currently online (in activeWorkers) or only seen via ledger entries
// (offline / never-seen-alive).
type KnownPeer struct {
	PeerID        peer.ID
	// PeerIDStr is the original string key used to look up the worker in
	// activeWorkers. For properly-encoded peer IDs it equals PeerID.String();
	// for legacy / test-injected raw-string keys it equals the raw string.
	PeerIDStr     string
	AgentName     string    // empty for never-seen-alive peers
	LastSeen      time.Time // zero for never-seen-alive peers
	IsOnline      bool
	HonestyScore  float64
	IsEquivocator bool
}

// ListKnownPeers returns every peer the boss has heard about, sorted with
// online peers first (newest-first by LastSeen), then offline by LastSeen
// desc. Uses activeWorkers for online status and store.DistinctSubjects for
// the rest. Decorates each entry with honesty score and equivocator flag.
func (b *Boss) ListKnownPeers(ctx context.Context) ([]KnownPeer, error) {
	// Key by raw string (not peer.ID) so reverse lookups into activeWorkers are exact.
	known := map[string]*KnownPeer{}

	b.mu.RLock()
	for pidStr, p := range b.activeWorkers {
		pid, err := peer.Decode(pidStr)
		if err != nil {
			// Fall back to treating the raw string as the peer.ID bytes.
			// This preserves backwards compatibility with tests that seed
			// workers with non-standard ID strings (e.g. "peer1").
			pid = peer.ID(pidStr)
		}
		ls := b.lastSeen[pidStr]
		known[pidStr] = &KnownPeer{
			PeerID:    pid,
			PeerIDStr: pidStr,
			AgentName: p.AgentName,
			LastSeen:  ls,
			IsOnline:  true,
		}
	}
	b.mu.RUnlock()

	if b.readStore != nil {
		subjects, err := b.readStore.DistinctSubjects(ctx)
		if err != nil {
			return nil, err
		}
		for _, pidBytes := range subjects {
			pid := peer.ID(pidBytes)
			pidStr := pid.String()
			if _, ok := known[pidStr]; ok {
				continue // already online — don't overwrite
			}
			known[pidStr] = &KnownPeer{PeerID: pid, PeerIDStr: pidStr, IsOnline: false}
		}
	}

	for _, kp := range known {
		if b.reputationEngine != nil {
			kp.HonestyScore = b.reputationEngine.Score(kp.PeerIDStr)
		}
		if b.ledger != nil {
			marked, _ := b.ledger.IsEquivocator(ctx, []byte(kp.PeerID))
			kp.IsEquivocator = marked
		}
	}

	out := make([]KnownPeer, 0, len(known))
	for _, kp := range known {
		out = append(out, *kp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsOnline != out[j].IsOnline {
			return out[i].IsOnline
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out, nil
}

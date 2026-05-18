package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
	"google.golang.org/protobuf/proto"
)

// defaultLedgerDBPath matches the existing convention for sibling
// state files (see internal/network/host.go: `.agentfm_<mode>_identity.key`).
const defaultLedgerDBPath = ".agentfm_ledger.db"

// runReputationSubcommand dispatches `agentfm reputation <action> [args...]`.
// Today only "show" is implemented; the parser is shaped for future
// "verify", "rehab", etc. subcommands (P3-7, P5-3).
func runReputationSubcommand(args []string) {
	if len(args) == 0 {
		pterm.Error.Println("Usage: agentfm reputation show <peer_id>")
		os.Exit(1)
	}
	switch args[0] {
	case "show":
		runReputationShow(args[1:])
	case "-h", "--help":
		printReputationHelp(os.Stdout)
	default:
		pterm.Error.Printfln("Unknown reputation subcommand: %s", args[0])
		printReputationHelp(os.Stderr)
		os.Exit(1)
	}
}

func printReputationHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: agentfm reputation <command> [args...]

Commands:
  show <peer_id>     Print scores, ledger head, and recent ratings about a peer.

Flags (apply to show):
  -db <path>         Path to the ledger SQLite database (default: .agentfm_ledger.db)
  -limit <N>         Number of most-recent entries to print (default: 20)`)
}

// runReputationShow gathers every inbox entry whose subject_peer_id
// matches the argument and prints a summary suitable for casual
// auditing. Output is human-targeted; for programmatic access use the
// HTTP API (P4-2).
func runReputationShow(args []string) {
	fs := flag.NewFlagSet("reputation show", flag.ExitOnError)
	dbPath := fs.String("db", defaultLedgerDBPath, "Path to the ledger SQLite database")
	limit := fs.Int("limit", 20, "Number of most-recent entries to print")
	fs.Usage = func() { printReputationHelp(os.Stderr) }

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	positional := fs.Args()
	if len(positional) == 0 {
		pterm.Error.Println("Usage: agentfm reputation show <peer_id>")
		os.Exit(1)
	}
	if len(positional) > 1 {
		pterm.Error.Printfln("show takes exactly one peer_id; got %d args", len(positional))
		os.Exit(1)
	}

	subjectIDStr := positional[0]
	subjectID, err := peer.Decode(subjectIDStr)
	if err != nil {
		pterm.Error.Printfln("invalid peer_id %q: %v", subjectIDStr, err)
		os.Exit(1)
	}

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		pterm.Error.Printfln("ledger database not found at %s — run a worker/boss in this directory first, or pass -db <path>", *dbPath)
		os.Exit(1)
	}

	s, err := store.Open(*dbPath)
	if err != nil {
		pterm.Error.Printfln("open ledger: %v", err)
		os.Exit(1)
	}
	defer s.Close()

	out, err := gatherReputationView(context.Background(), s, []byte(subjectID), *limit)
	if err != nil {
		pterm.Error.Printfln("gather reputation: %v", err)
		os.Exit(1)
	}
	renderReputationView(os.Stdout, out)
}

// reputationView is a packaged set of facts about a subject peer,
// extracted from the local inbox. Held in its own type so the
// renderer can be tested without an open SQLite handle.
type reputationView struct {
	Subject       peer.ID
	EntryCount    int
	LatestEntries []reputationRow
	LastSeen      time.Time
}

type reputationRow struct {
	ReceivedAt time.Time
	Rater      peer.ID
	Dimension  string
	Score      float64
	Context    string
	Kind       string // "Rating" or "Comment"
}

// gatherReputationView scans BOTH the boss's own log (entries table)
// AND the inbox (entries table populated by gossip from other peers),
// keeping the rows whose subject matches the requested peer.
//
// Why both: when this CLI runs against a Boss's ledger DB, the
// attestation ratings the Boss issued live in `entries`. When the
// CLI runs against a Worker's ledger (or anyone else's), the
// remote-peer ratings live in `inbox_entries`. Reading both means
// the CLI shows the complete picture regardless of which ledger
// it's pointed at.
//
// The in-Go filter is acceptable for v1.3 demo scale (≤ ~100k
// entries); a column index can be added later if needed.
func gatherReputationView(ctx context.Context, s *store.Store, subjectID []byte, limit int) (*reputationView, error) {
	if limit < 0 {
		limit = 0
	}

	view := &reputationView{Subject: peer.ID(subjectID)}

	collect := func(payload []byte, receivedAtNs int64) {
		var signed pb.SignedEntry
		if err := proto.Unmarshal(payload, &signed); err != nil {
			return // skip malformed
		}
		row, ok := rowFromEntry(&signed, receivedAtNs)
		if !ok {
			return
		}
		if !bytesEqual(row.subjectPeerID, subjectID) {
			return
		}
		view.EntryCount++
		if row.ReceivedAt.After(view.LastSeen) {
			view.LastSeen = row.ReceivedAt
		}
		view.LatestEntries = append(view.LatestEntries, row.reputationRow)
	}

	// Own log first.
	if err := s.IterateAllOwnEntries(ctx, func(e *store.Entry) error {
		collect(e.Payload, e.InsertedAt)
		return nil
	}); err != nil {
		return nil, err
	}
	// Then inbox (gossip from other peers).
	if err := s.IterateAllInboxEntries(ctx, func(e *store.InboxEntry) error {
		collect(e.Payload, e.ReceivedAt)
		return nil
	}); err != nil {
		return nil, err
	}

	// Sort newest-first, then truncate to the configured limit.
	sort.Slice(view.LatestEntries, func(i, j int) bool {
		return view.LatestEntries[i].ReceivedAt.After(view.LatestEntries[j].ReceivedAt)
	})
	if limit > 0 && len(view.LatestEntries) > limit {
		view.LatestEntries = view.LatestEntries[:limit]
	}
	return view, nil
}

// extractedRow bundles the subject_peer_id alongside the renderable
// row so the gather loop can filter cheaply before keeping the row.
type extractedRow struct {
	reputationRow
	subjectPeerID []byte
}

// rowFromEntry pulls the fields the CLI cares about out of either
// oneof variant. Returns (_, false) for malformed input — caller skips.
func rowFromEntry(signed *pb.SignedEntry, receivedAtNs int64) (extractedRow, bool) {
	receivedAt := time.Unix(0, receivedAtNs)
	switch body := signed.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating == nil {
			return extractedRow{}, false
		}
		return extractedRow{
			reputationRow: reputationRow{
				ReceivedAt: receivedAt,
				Rater:      peer.ID(body.Rating.RaterPeerId),
				Dimension:  body.Rating.Dimension,
				Score:      body.Rating.Score,
				Context:    body.Rating.Context,
				Kind:       "Rating",
			},
			subjectPeerID: body.Rating.SubjectPeerId,
		}, true
	case *pb.SignedEntry_Comment:
		if body.Comment == nil {
			return extractedRow{}, false
		}
		return extractedRow{
			reputationRow: reputationRow{
				ReceivedAt: receivedAt,
				Rater:      peer.ID(body.Comment.RaterPeerId),
				Dimension:  "(comment)",
				Context:    body.Comment.Language,
				Kind:       "Comment",
			},
			subjectPeerID: body.Comment.SubjectPeerId,
		}, true
	default:
		return extractedRow{}, false
	}
}

// renderReputationView writes the human-targeted view to w. Split out
// from runReputationShow so tests can assert against the rendered
// output without spawning a subprocess.
func renderReputationView(w io.Writer, v *reputationView) {
	fmt.Fprintf(w, "Peer:       %s\n", v.Subject.String())
	if v.EntryCount == 0 {
		fmt.Fprintln(w, "Entries:    0 (no ratings about this peer in the local inbox)")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Honesty:    [no data]")
		return
	}
	fmt.Fprintf(w, "Entries:    %d (last: %s)\n", v.EntryCount, v.LastSeen.UTC().Format(time.RFC3339))
	// Note: the CLI shows raw rating history. For the live
	// EigenTrust-aggregated honesty score (with seed weighting +
	// age decay), hit the HTTP API instead:
	//   curl http://<gateway>/v1/peers/<peer_id>/reputation
	fmt.Fprintln(w, "Honesty:    (raw rating list below; aggregated score via /v1/peers/{id}/reputation)")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Latest:")
	for _, r := range v.LatestEntries {
		ageStr := compactAge(time.Since(r.ReceivedAt))
		switch r.Kind {
		case "Rating":
			fmt.Fprintf(w, "            %+.2f %s by %s (%s) %s ago\n",
				r.Score, r.Dimension, shortPeer(r.Rater), nonEmpty(r.Context, "no-context"), ageStr)
		case "Comment":
			fmt.Fprintf(w, "            comment by %s (lang=%s) %s ago\n",
				shortPeer(r.Rater), nonEmpty(r.Context, "?"), ageStr)
		}
	}
}

// shortPeer trims a libp2p peer ID to its first/last 6 chars for
// log-friendliness: "12D3Ko...8s5zL".
func shortPeer(id peer.ID) string {
	s := id.String()
	if len(s) <= 16 {
		return s
	}
	return s[:6] + "..." + s[len(s)-5:]
}

// compactAge formats a duration as the largest unit that fits
// (compact, log-style: "12s", "4m", "3h", "2d"). For ages too far in
// the future (clock skew) returns "soon".
func compactAge(d time.Duration) string {
	if d < 0 {
		return "soon"
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

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func bytesEqual(a, b []byte) bool {
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

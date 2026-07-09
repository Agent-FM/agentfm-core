package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"

	"github.com/pterm/pterm"
)

// runRelayMode brings up a permanent lighthouse: Circuit Relay v2 with
// infinite reservation limits, a Kademlia DHT in server mode, an actively
// drained telemetry subscription (so it keeps routing gossip), and a full
// archive ledger. The archive persists every signed Rating / Comment /
// EquivocationAlert, serves head-fetch / ledger-fetch, and replicates +
// serves comment bodies (comment-fetch), so a fresh boss can catch up
// against this relay even when every other boss is offline.
//
// This is the single relay path — the dedicated relay binary was folded in
// here so `agentfm -mode relay` is the only relay, dev or production.
func runRelayMode(ctx context.Context, netCfg network.Config, promListen string) {
	relayCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	node, err := network.Setup(relayCtx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}

	if promListen != "" {
		go func() {
			if err := metrics.Serve(relayCtx, promListen); err != nil {
				pterm.Error.Printfln("metrics server: %v", err)
			}
		}()
		pterm.Success.Printfln("Metrics server: http://%s/metrics", promListen)
	}

	// Actively route telemetry: join + subscribe + drain. If we never call
	// Next on the subscription its buffer fills and the topic stops
	// forwarding to other peers. The loop exits when relayCtx is cancelled.
	if topic, err := node.PubSub.Join(network.TelemetryTopic); err != nil {
		slog.Warn("relay: could not join telemetry topic; gossip routing degraded",
			slog.Any(obs.FieldErr, err))
	} else if sub, err := topic.Subscribe(); err != nil {
		slog.Warn("relay: telemetry subscribe failed", slog.Any(obs.FieldErr, err))
	} else {
		go func() {
			for {
				if _, err := sub.Next(relayCtx); err != nil {
					return
				}
			}
		}()
	}

	// Archive ledger: signs with the same identity the host uses, so the
	// ledger's authored-by matches this relay's peer id.
	idPath := netCfg.IdentityPath
	if idPath == "" {
		idPath = resolveRelayIdentityPath("")
	}
	priv, err := network.LoadOrGenerateIdentity(idPath)
	if err != nil {
		pterm.Fatal.Printfln("relay identity load failed: %v", err)
	}

	var cstore *comments.Store
	if cs, err := comments.Open(defaultCommentsRoot()); err != nil {
		slog.Warn("relay: comments store open failed; comment bodies will not be archived",
			slog.Any(obs.FieldErr, err))
	} else {
		cstore = cs
		cserver := comments.NewServer(node.Host, cstore)
		cserver.Start()
		defer cserver.Stop()
	}

	dbPath := defaultRelayLedgerPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		slog.Warn("relay: could not create ledger dir; running connectivity-only",
			slog.String("path", filepath.Dir(dbPath)), slog.Any(obs.FieldErr, err))
	} else if arch, err := ledger.NewWithOptions(dbPath, priv, node.PubSub, ledger.Options{Host: node.Host, Comments: cstore}); err != nil {
		slog.Warn("relay: archive ledger failed to open; running connectivity-only",
			slog.Any(obs.FieldErr, err))
	} else {
		defer func() { _ = arch.Close() }()
		if head, err := arch.Head(relayCtx); err == nil && head != nil {
			fmt.Printf("📚 Relay archive ledger opened at %s (tree_size=%d)\n", dbPath, head.TreeSize)
		} else {
			fmt.Printf("📚 Relay archive ledger opened at %s (empty)\n", dbPath)
		}
	}

	pterm.Success.Println("Relay Node Active — Circuit Relay v2 + DHT server + ledger archive. Press Ctrl+C to shut down.")
	pterm.Info.Printfln("Peer ID: %s", node.Host.ID().String())
	pterm.Info.Println("📌 Connect nodes to this relay with -bootstrap:")
	for _, addr := range node.Host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr.String(), node.Host.ID().String())
	}

	<-relayCtx.Done()
	fmt.Println("\nShutting down relay...")
}

// resolveRelayIdentityPath returns the persistent libp2p key file for a
// relay. An explicit -identity wins; otherwise ~/.agentfm/relay_identity.key
// (creating ~/.agentfm) so the relay's peer id — and the bootstrap multiaddr
// nodes dial — stays stable across restarts regardless of the launch dir.
func resolveRelayIdentityPath(override string) string {
	if override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentfm_relay_identity.key"
	}
	dir := filepath.Join(home, ".agentfm")
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "relay_identity.key")
}

// defaultRelayLedgerPath returns ~/.agentfm/relay_ledger.db. Disjoint from
// the boss/api/witness ledger DBs so co-located processes don't clobber it.
func defaultRelayLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentfm/relay_ledger.db"
	}
	return filepath.Join(home, ".agentfm", "relay_ledger.db")
}

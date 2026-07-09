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

// runWitnessMode brings up a ledger-only replica node. The witness
// joins the same libp2p mesh as bosses, subscribes to FeedbackTopic
// + EquivocationTopic via the ledger engine, persists every signed
// entry to its own SQLite at ~/.agentfm/witness_ledger.db, and
// serves head-fetch / ledger-fetch / comment-fetch streams so a
// fresh boss can catch up against it even when no other boss is
// online.
//
// A witness exposes no HTTP API, runs no podman, and does not sign
// outgoing entries. Co-signing of LogHeads is a separate follow-up.
func runWitnessMode(ctx context.Context, netCfg network.Config, promListen string) {
	witnessCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	node, err := network.Setup(witnessCtx, netCfg)
	if err != nil {
		pterm.Fatal.Println(err)
	}

	if promListen != "" {
		go func() {
			if err := metrics.Serve(witnessCtx, promListen); err != nil {
				pterm.Error.Printfln("metrics server: %v", err)
			}
		}()
		pterm.Success.Printfln("Metrics server: http://%s/metrics", promListen)
	}

	keyPath := defaultWitnessIdentityPath()
	priv, err := network.LoadOrGenerateIdentity(keyPath)
	if err != nil {
		pterm.Fatal.Printfln("witness identity load failed: %v", err)
	}

	dbPath := defaultWitnessLedgerPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		pterm.Fatal.Printfln("cannot create ledger dir %s: %v", dbPath, err)
	}

	cstore, err := comments.Open(defaultCommentsRoot())
	if err != nil {
		slog.Warn("witness: comments store open failed; CommentFetch disabled",
			slog.Any(obs.FieldErr, err))
	}

	l, err := ledger.NewWithOptions(dbPath, priv, node.PubSub, ledger.Options{
		Host:     node.Host,
		Comments: cstore,
	})
	if err != nil {
		pterm.Fatal.Printfln("ledger open failed: %v", err)
	}
	defer func() { _ = l.Close() }()
	slog.Info("witness: ledger opened", slog.String("path", dbPath))

	if cstore != nil {
		cserver := comments.NewServer(node.Host, cstore)
		cserver.Start()
		defer cserver.Stop()
		slog.Info("witness: comments store opened",
			slog.String("path", defaultCommentsRoot()))
	}

	pterm.Success.Println("Witness Node Active. Press Ctrl+C to shut down.")
	pterm.Info.Printfln("Peer ID: %s", node.Host.ID().String())
	pterm.Info.Println("📌 Bosses joining this swarm will catch-up against this witness:")
	for _, addr := range node.Host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr.String(), node.Host.ID().String())
	}

	<-witnessCtx.Done()
	fmt.Println("\nShutting down witness...")
}

// defaultWitnessLedgerPath returns ~/.agentfm/witness_ledger.db.
// Disjoint from boss/api/relay ledger DBs so a witness co-located
// with a boss process on the same machine doesn't clobber state.
func defaultWitnessLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentfm/witness_ledger.db"
	}
	return filepath.Join(home, ".agentfm", "witness_ledger.db")
}

// defaultWitnessIdentityPath returns the libp2p private-key file
// used to derive the witness's stable peer id. Persisting it under
// ~/.agentfm (home-scoped, like the relay) keeps the witness peer id
// stable across restarts regardless of the launch directory — matching
// the home-scoped witness ledger DB.
func defaultWitnessIdentityPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".agentfm_witness_identity.key"
	}
	return filepath.Join(home, ".agentfm", "witness_identity.key")
}

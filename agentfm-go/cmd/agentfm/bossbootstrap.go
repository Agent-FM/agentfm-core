package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"agentfm/internal/boss"
	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/reputation"
	"agentfm/internal/trustedagents"
)

// bossOptionsFromFlags assembles the v1.3 boss.Options bundle that
// wires the ledger, trusted-agents registry, comments store,
// reputation engine, and attestation policy. Used by runBossMode +
// runAPIMode so the v1.3 HTTP / ledger surfaces are LIVE in the
// running binary (rather than returning 503 ledger_unavailable).
//
// The function is best-effort: every component is optional. A
// failure to open the ledger (e.g. permission denied on the SQLite
// path) is logged and the boss boots without ledger-backed
// endpoints — the rest of the gateway still works.
//
// The returned cleanup func MUST be called at shutdown.
func bossOptionsFromFlags(
	ctx context.Context,
	mode string,
	node *network.MeshNode,
	attestModeFlag string,
	rejectUnknownImages bool,
	trustedAgentsPath string,
	genesisSeedsPath string,
) (boss.Options, func()) {
	opts := boss.Options{
		AttestationMode:     boss.ParseAttestationMode(attestModeFlag),
		RejectUnknownImages: rejectUnknownImages,
	}
	cleanups := []func(){}
	cleanup := func() {
		// Run cleanups in reverse order (LIFO).
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// --- trusted-agents registry --------------------------------------
	if reg, err := trustedagents.LoadFile(trustedAgentsPath); err == nil {
		opts.TrustedAgents = reg
	} else {
		slog.Warn("boss bootstrap: trusted-agents load failed; running with empty registry",
			slog.Any(obs.FieldErr, err))
	}

	// --- ledger -------------------------------------------------------
	keyPath := fmt.Sprintf(".agentfm_%s_identity.key", mode)
	priv, err := network.LoadOrGenerateIdentity(keyPath)
	if err != nil {
		slog.Warn("boss bootstrap: identity load failed; ledger disabled",
			slog.Any(obs.FieldErr, err))
		return opts, cleanup
	}

	dbPath := defaultBossLedgerPath(mode)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		slog.Warn("boss bootstrap: cannot create ledger dir; ledger disabled",
			slog.String("path", dbPath),
			slog.Any(obs.FieldErr, err))
		return opts, cleanup
	}

	l, err := ledger.NewWithOptions(dbPath, priv, node.PubSub, ledger.Options{
		Host: node.Host,
	})
	if err != nil {
		slog.Warn("boss bootstrap: ledger open failed; v1.3 endpoints will 503",
			slog.String("path", dbPath),
			slog.Any(obs.FieldErr, err))
		return opts, cleanup
	}
	opts.Ledger = l
	cleanups = append(cleanups, func() { _ = l.Close() })
	slog.Info("boss bootstrap: ledger opened",
		slog.String("path", dbPath))

	// Open a SECOND store handle on the same DB file for the
	// reputation engine's read-only walks. SQLite under WAL mode
	// supports concurrent open handles cleanly; the engine never
	// writes, the ledger always does. Avoids reaching into the
	// ledger impl for its private store reference.
	readStore, err := store.Open(dbPath)
	if err != nil {
		slog.Warn("boss bootstrap: secondary store open failed; reputation engine disabled",
			slog.Any(obs.FieldErr, err))
	} else {
		cleanups = append(cleanups, func() { _ = readStore.Close() })
		opts.ReadStore = readStore // wired through so HTTP handler can recompute on demand
	}

	// --- comments store + submission handler --------------------------
	cstore, err := comments.Open(defaultCommentsRoot())
	if err != nil {
		slog.Warn("boss bootstrap: comments store open failed; P4-3 disabled",
			slog.Any(obs.FieldErr, err))
	} else {
		cserver := comments.NewServer(node.Host, cstore)
		cserver.Start()
		cleanups = append(cleanups, cserver.Stop)

		// The boss's POST /v1/peers/{id}/comments handler needs a
		// reference to the boss, but the boss hasn't been
		// constructed yet (we're producing its Options). Use a
		// late-binding closure — the bootstrap caller calls
		// AttachBoss(b) right after boss.NewWithOptions returns.
		handler := boss.NewCommentSubmissionHandler(cstore, node.Host)
		opts.CommentSubmissionHandler = func(w http.ResponseWriter, r *http.Request) {
			handler.HandleHTTP(currentBossRef.Load(), w, r)
		}
	}

	// --- reputation engine + recompute ticker -------------------------
	if readStore != nil {
		seeds, err := reputation.LoadSeedsFile(genesisSeedsPath)
		if err != nil {
			slog.Warn("boss bootstrap: genesis seeds load failed; using bundled defaults",
				slog.Any(obs.FieldErr, err))
			seeds, _ = reputation.LoadDefaultSeeds()
		}
		// Self-seed: this boss's OWN peer id gets score 1.0 in its
		// OWN reputation engine. EigenTrust's mathematical premise
		// is "a rater's voting weight equals their own current
		// reputation"; without self-seeding, a fresh boss's
		// machine-issued attestation ratings have zero voting
		// weight and don't move scores. From the boss's local
		// perspective, trusting your own attestation gate fully is
		// the natural fixed point — and other peers in the mesh
		// don't automatically inherit this trust (they have to
		// accumulate evidence about this boss through OTHER seeds
		// independently).
		seeds = append(seeds, reputation.Seed{
			PeerID: node.Host.ID().String(),
			Score:  1.0,
		})
		engine := reputation.New(seeds, reputation.Config{})
		opts.ReputationEngine = engine

		// Initial recompute so the first request has fresh data.
		if _, err := engine.Recompute(ctx, readStore); err != nil {
			slog.Debug("boss bootstrap: initial reputation recompute failed",
				slog.Any(obs.FieldErr, err))
		}

		// Background recompute — every 60s per P5-1.
		tickCtx, tickCancel := context.WithCancel(context.Background())
		go runReputationTicker(tickCtx, engine, readStore)
		cleanups = append(cleanups, tickCancel)
	}

	return opts, cleanup
}

// runReputationTicker runs the engine's 60s recompute loop. Exits
// on ctx cancel; tolerates Recompute errors (logs at debug, retries
// next tick).
func runReputationTicker(ctx context.Context, eng *reputation.Engine, s *store.Store) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := eng.Recompute(ctx, s); err != nil {
				slog.Debug("reputation: recompute tick failed",
					slog.Any(obs.FieldErr, err))
			}
		}
	}
}

// defaultBossLedgerPath returns ~/.agentfm/<mode>_ledger.db.
// Falls back to working dir if HOME isn't set (CI sandboxes).
func defaultBossLedgerPath(mode string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Sprintf(".agentfm_%s_ledger.db", mode)
	}
	return filepath.Join(home, ".agentfm", fmt.Sprintf("%s_ledger.db", mode))
}

// defaultCommentsRoot returns ~/.agentfm/comments (or local fallback).
func defaultCommentsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".agentfm_comments"
	}
	return filepath.Join(home, ".agentfm", "comments")
}

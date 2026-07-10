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

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// bossOptionsFromFlags assembles the v1.3 boss.Options bundle that
// wires the ledger, comments store, reputation engine, and floor policy.
// Used by runBossMode + runAPIMode so the v1.3 HTTP / ledger surfaces are
// LIVE in the running binary (rather than returning 503 ledger_unavailable).
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
	reputationFloor float64,
	genesisSeedsPath string,
	ledgerPathOverride string,
) (boss.Options, func()) {
	opts := boss.Options{
		// Always pass a pointer so an explicit --reputation-floor=0 stays 0
		// and does not collide with the "unconfigured" sentinel.
		ReputationFloor: &reputationFloor,
	}
	cleanups := []func(){}
	cleanup := func() {
		// Run cleanups in reverse order (LIFO).
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// --- ledger -------------------------------------------------------
	// Scope the signing identity to ~/.agentfm (like the ledger DB) so a
	// boss launched from a different directory keeps the same own-log
	// author. An existing cwd-relative key (older layout) is preferred so
	// upgrading doesn't silently re-key an established log.
	keyPath := defaultBossIdentityPath(mode)
	legacyKeyPath := fmt.Sprintf(".agentfm_%s_identity.key", mode)
	if _, statErr := os.Stat(legacyKeyPath); statErr == nil {
		keyPath = legacyKeyPath
	}
	priv, err := network.LoadOrGenerateIdentity(keyPath)
	if err != nil {
		slog.Warn("boss bootstrap: identity load failed; ledger disabled",
			slog.Any(obs.FieldErr, err))
		return opts, cleanup
	}

	dbPath := defaultBossLedgerPath(mode)
	if ledgerPathOverride != "" {
		dbPath = ledgerPathOverride
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		slog.Warn("boss bootstrap: cannot create ledger dir; ledger disabled",
			slog.String("path", dbPath),
			slog.Any(obs.FieldErr, err))
		return opts, cleanup
	}

	cstore, err := comments.Open(defaultCommentsRoot())
	if err != nil {
		slog.Warn("boss bootstrap: comments store open failed; P4-3 disabled",
			slog.Any(obs.FieldErr, err))
	}

	l, err := ledger.NewWithOptions(dbPath, priv, node.PubSub, ledger.Options{
		Host:     node.Host,
		Comments: cstore,
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

	// Cancellable context for the background catch-up + hourly rater so
	// shutdown signals them to stop. cleanups run LIFO, so appending this
	// cancel AFTER the l.Close cleanup guarantees it fires before the
	// ledger is closed (avoiding "database is closed" errors on the bg
	// paths that write through l).
	bootstrapCtx, bootstrapCancel := context.WithCancel(ctx)
	cleanups = append(cleanups, bootstrapCancel)

	// P5-1: on restart, pull any entries the boss missed while offline.
	// Non-fatal: if catch-up fails the boss continues normally.
	go func() {
		const waitBudget = 30 * time.Second
		deadline := time.Now().Add(waitBudget)

		poll := time.NewTicker(1 * time.Second)
		defer poll.Stop()
		for time.Now().Before(deadline) {
			if node.RelayPeerID != "" &&
				node.Host.Network().Connectedness(node.RelayPeerID) == netcore.Connected {
				break
			}
			select {
			case <-bootstrapCtx.Done():
				return
			case <-poll.C:
			}
		}
		if node.RelayPeerID == "" ||
			node.Host.Network().Connectedness(node.RelayPeerID) != netcore.Connected {
			slog.Warn("boss bootstrap: relay never came online within 30s; no catch-up")
			return
		}

		// libp2p's identify protocol runs AFTER Connectedness flips to
		// Connected. Without a small settle delay, h.NewStream(...) for a
		// non-baseline protocol (head-fetch, ledger-fetch, inbox-fetch)
		// can race ahead of the remote's identify response and fail with
		// "protocols not supported" — silently truncating catch-up to zero
		// entries. Two seconds is empirically enough on TCP loopback +
		// public lighthouse; cheap insurance on a 30s budget.
		select {
		case <-bootstrapCtx.Done():
			return
		case <-time.After(2 * time.Second):
		}

		bgCtx, bgCancel := context.WithTimeout(bootstrapCtx, 2*time.Minute)
		defer bgCancel()

		if err := ledger.CatchUp(bgCtx, l, node.Host, node.RelayPeerID); err != nil {
			slog.Warn("boss bootstrap: own-log catch-up against relay failed",
				slog.Any(obs.FieldErr, err))
		} else {
			slog.Info("boss bootstrap: own-log catch-up complete")
		}

		candidates := []peer.ID{node.RelayPeerID}
		seen := map[peer.ID]struct{}{node.RelayPeerID: {}}
		for _, p := range node.Host.Network().Peers() {
			if _, ok := seen[p]; ok {
				continue
			}
			if p == node.Host.ID() {
				continue
			}
			seen[p] = struct{}{}
			candidates = append(candidates, p)
			if len(candidates) >= 5 {
				break
			}
		}
		for _, p := range candidates {
			if err := ledger.CatchUpInbox(bgCtx, l, node.Host, p); err != nil {
				slog.Debug("boss bootstrap: inbox catch-up against peer failed",
					slog.String("peer", p.String()),
					slog.Any(obs.FieldErr, err))
				continue
			}
			slog.Info("boss bootstrap: inbox catch-up complete",
				slog.String("peer", p.String()))
		}
	}()

	// Hourly aggregate outcome rater (P2 / Task 2.1).
	// RunTicker must be started after the boss is constructed; the
	// bootstrap caller is responsible for: go opts.CompletionRater.RunTicker(ctx).
	// We store the ticker in opts so the caller has a reference.
	opts.CompletionRater = boss.NewCompletionRatingWriter(l, node.Host)
	// Start the ticker in the background; cancelled on shutdown via
	// bootstrapCancel (registered as a cleanup above) so it exits even
	// when the caller passed a non-cancellable ctx (e.g. api mode).
	go opts.CompletionRater.RunTicker(bootstrapCtx)

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
	// cstore was opened before the ledger so Options.Comments could be
	// wired; nil here means the open failed and P4-3 stays disabled.
	if cstore != nil {
		cserver := comments.NewServer(node.Host, cstore)
		cserver.Start()
		cleanups = append(cleanups, cserver.Stop)

		// Wire commentsStore so GET /v1/peers/{id}/comments/{cid}
		// can hydrate comment bodies (sub-task 1.5 / Phase 1).
		opts.CommentsStore = cstore

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

// defaultBossIdentityPath returns the home-scoped signing-identity key
// path for a boss/api ledger, matching defaultBossLedgerPath so the key
// and DB live together and the own-log author stays stable across launch
// directories.
func defaultBossIdentityPath(mode string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Sprintf(".agentfm_%s_identity.key", mode)
	}
	return filepath.Join(home, ".agentfm", fmt.Sprintf("%s_identity.key", mode))
}

// defaultCommentsRoot returns ~/.agentfm/comments (or local fallback).
func defaultCommentsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".agentfm_comments"
	}
	return filepath.Join(home, ".agentfm", "comments")
}

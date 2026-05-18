package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"agentfm/internal/ledger"
	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/pterm/pterm"
)

// fatalf prints a styled error line and exits. Using the same pterm.Fatal
// path as cmd/agentfm means operators get a consistent startup UX across
// both binaries, and avoids the goroutine stack dump that panic produces.
func fatalf(format string, args ...interface{}) {
	pterm.Fatal.Printfln(format, args...)
}

// homeDir returns the current user's home directory, falling back to "."
// if os.UserHomeDir fails (e.g. in a sandboxed or container environment).
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "."
}

// getStaticIdentity is a thin wrapper around network.LoadOrGenerateIdentity
// kept for the cmd/relay binary's existing logging idiom. The shared helper
// takes care of the corrupt-file warning, the 0600 perm, and the
// regenerate-on-missing semantics so both binaries stay in lockstep.
func getStaticIdentity(keyFile string) (crypto.PrivKey, error) {
	return network.LoadOrGenerateIdentity(keyFile)
}

func main() {
	port := flag.Int("port", 4001, "Port to listen on")
	swarmKey := flag.String("swarmkey", "", "Path to private swarm.key file (optional)")
	identityFile := flag.String("identity", "relay_identity.key", "File to save/load the node identity")
	promListen := flag.String("prom-listen", "127.0.0.1:9091", "Prometheus /metrics listen address (loopback by default; pass - to disable)")
	logFormat := flag.String("log-format", obs.FormatAuto, "Log format: json, console, auto")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	// Relays are the obvious place to host the witness co-sign service:
	// long-lived, stable PeerID, low task load. Default on so a fresh
	// relay deploy contributes to the v1.3 witness pool automatically.
	witness := flag.Bool("witness", true, "Serve the witness co-sign role on this relay (v1.3 verifiable mesh)")
	flag.Parse()
	_ = witness // wired by P2-2's handler registration

	obs.Init("relay", *logFormat, *logLevel)

	fmt.Println("🚀 Starting AgentFM Permanent Lighthouse...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	privKey, err := getStaticIdentity(*identityFile)
	if err != nil {
		fatalf("failed to load identity: %v", err)
	}

	// Configure the libp2p Host options
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port),
			fmt.Sprintf("/ip6/::/tcp/%d", *port),
		),
		libp2p.EnableRelayService(relay.WithInfiniteLimits()),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableNATService(),
	}

	// When an operator passes a swarm key this relay becomes a private
	// darknet lighthouse. We reuse network.LoadSwarmKey so both binaries
	// parse the PSK file through exactly the same code path.
	if *swarmKey != "" {
		psk, err := network.LoadSwarmKey(*swarmKey)
		if err != nil {
			fatalf("failed to load swarm key: %v", err)
		}
		opts = append(opts, libp2p.PrivateNetwork(psk))
		fmt.Println("🔒 PRIVATE SWARM MODE ENABLED: This relay will drop all public internet traffic.")
	}

	// Boot the Host
	host, err := libp2p.New(opts...)
	if err != nil {
		fatalf("failed to create libp2p host: %v", err)
	}

	ps, err := pubsub.NewGossipSub(
		ctx,
		host,
		pubsub.WithFloodPublish(true),
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		fatalf("failed to create gossipsub: %v", err)
	}

	topic, err := ps.Join(network.TelemetryTopic)
	if err != nil {
		fatalf("failed to join telemetry topic: %v", err)
	}
	defer func() { _ = topic.Close() }()

	// Subscribe so this relay actively routes messages. If we never call
	// Next on the subscription its buffer fills and the topic stops
	// forwarding to other peers.
	sub, err := topic.Subscribe()
	if err != nil {
		fatalf("failed to subscribe: %v", err)
	}
	defer sub.Cancel()

	// Drain the subscription in the background so the relay keeps
	// forwarding telemetry instead of letting the topic buffer fill.
	// The loop exits cleanly when ctx is cancelled during shutdown.
	go func() {
		for {
			if _, err := sub.Next(ctx); err != nil {
				return
			}
		}
	}()

	// v1.3.1: open a full ledger archive. The constructor auto-subscribes to
	// FeedbackTopic and EquivocationTopic, and auto-registers the
	// LedgerFetchProtocol handler — so signed Rating + Comment + EquivocationAlert
	// entries get archived in SQLite and served to peers catching up after restart.
	ledgerPath := filepath.Join(homeDir(), ".agentfm", "relay_ledger.db")
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o700); err != nil {
		slog.Warn("relay: could not create ledger directory",
			slog.String("path", filepath.Dir(ledgerPath)),
			slog.Any(obs.FieldErr, err))
	} else {
		arch, err := ledger.NewWithOptions(ledgerPath, privKey, ps, ledger.Options{Host: host})
		if err != nil {
			slog.Warn("relay: archive ledger failed to open; running as connectivity-only relay",
				slog.Any(obs.FieldErr, err))
		} else {
			defer func() { _ = arch.Close() }()
			if head, err := arch.Head(ctx); err == nil && head != nil {
				fmt.Printf("📚 Relay archive ledger opened at %s (tree_size=%d)\n", ledgerPath, head.TreeSize)
			} else {
				fmt.Printf("📚 Relay archive ledger opened at %s (empty)\n", ledgerPath)
			}
		}
	}

	// Start the Kademlia DHT in Server Mode
	kDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		fatalf("failed to create DHT: %v", err)
	}
	if err = kDHT.Bootstrap(ctx); err != nil {
		fatalf("failed to bootstrap DHT: %v", err)
	}

	if listen := *promListen; listen != "" && listen != "-" {
		go func() {
			if err := metrics.Serve(ctx, listen); err != nil {
				pterm.Error.Printfln("metrics server: %v", err)
			}
		}()
		fmt.Printf("📊 Metrics server: http://%s/metrics\n", listen)
	}

	fmt.Println("\n✅ Permanent Relay Node is Online!")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("THIS IS YOUR FOREVER ADDRESS (Port %d):\n", *port)
	for _, addr := range host.Addrs() {
		fmt.Printf("%s/p2p/%s\n", addr.String(), host.ID().String())
	}
	fmt.Println("--------------------------------------------------")
	fmt.Println("Press CTRL+C to stop the server.")

	<-ctx.Done()

	fmt.Println("\nShutting down relay node...")
	if err := host.Close(); err != nil {
		// slog (not pterm) so log shippers see structured failure events
		// for relays running unattended under systemd / docker / k8s.
		slog.Error("relay host close",
			slog.Any(obs.FieldErr, err),
		)
	}
}

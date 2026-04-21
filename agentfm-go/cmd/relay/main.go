package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agentfm/internal/network"

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

// This function loads a saved identity or generates a new one
func getStaticIdentity(keyFile string) (crypto.PrivKey, error) {
	if keyBytes, err := os.ReadFile(keyFile); err == nil {
		fmt.Printf("🔑 Loaded existing permanent identity from %s!\n", keyFile)
		return crypto.UnmarshalPrivateKey(keyBytes)
	}

	fmt.Printf("✨ Generating new permanent identity at %s...\n", keyFile)
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, err
	}

	keyBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(keyFile, keyBytes, 0600)
	return priv, err
}

func main() {
	port := flag.Int("port", 4001, "Port to listen on")
	swarmKey := flag.String("swarmkey", "", "Path to private swarm.key file (optional)")
	identityFile := flag.String("identity", "relay_identity.key", "File to save/load the node identity")
	flag.Parse()

	fmt.Println("🚀 Starting AgentFM Permanent Lighthouse...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Subscribe so this relay actively routes messages. If we never call
	// Next on the subscription its buffer fills and the topic stops
	// forwarding to other peers.
	sub, err := topic.Subscribe()
	if err != nil {
		fatalf("failed to subscribe: %v", err)
	}

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

	// Start the Kademlia DHT in Server Mode
	kDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		fatalf("failed to create DHT: %v", err)
	}
	if err = kDHT.Bootstrap(ctx); err != nil {
		fatalf("failed to bootstrap DHT: %v", err)
	}

	fmt.Println("\n✅ Permanent Relay Node is Online!")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("THIS IS YOUR FOREVER ADDRESS (Port %d):\n", *port)
	for _, addr := range host.Addrs() {
		fmt.Printf("%s/p2p/%s\n", addr.String(), host.ID().String())
	}
	fmt.Println("--------------------------------------------------")
	fmt.Println("Press CTRL+C to stop the server.")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	fmt.Println("\nShutting down relay node...")
	if err := host.Close(); err != nil {
		pterm.Error.Printfln("Host close error: %v", err)
	}
}

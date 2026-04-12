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
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

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

func loadSwarmKey(path string) (pnet.PSK, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open swarm key file: %w", err)
	}
	defer file.Close()

	psk, err := pnet.DecodeV1PSK(file)
	if err != nil {
		return nil, fmt.Errorf("could not decode swarm key: %w", err)
	}
	return psk, nil
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
		panic(fmt.Errorf("failed to load identity: %w", err))
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

	//  If a swarm key is provided, turn this into a Private Darknet Relay
	if *swarmKey != "" {
		psk, err := loadSwarmKey(*swarmKey)
		if err != nil {
			panic(fmt.Errorf("failed to load swarm key: %w", err))
		}
		opts = append(opts, libp2p.PrivateNetwork(psk))
		fmt.Println("🔒 PRIVATE SWARM MODE ENABLED: This relay will drop all public internet traffic.")
	}

	// Boot the Host
	host, err := libp2p.New(opts...)
	if err != nil {
		panic(fmt.Errorf("failed to create libp2p host: %w", err))
	}

	ps, err := pubsub.NewGossipSub(
		ctx,
		host,
		pubsub.WithFloodPublish(true),
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create gossipsub: %w", err))
	}

	topic, err := ps.Join(network.TelemetryTopic)
	if err != nil {
		panic(fmt.Errorf("failed to join telemetry topic: %w", err))
	}

	// Subscribe to force the Relay to actively route messages!
	sub, err := topic.Subscribe()
	if err != nil {
		panic(fmt.Errorf("failed to subscribe: %w", err))
	}

	// Run a background loop to constantly "read" the messages so the Relay's memory doesn't fill up
	go func() {
		for {
			_, err := sub.Next(ctx)
			if err != nil {
				break
			}
		}
	}()

	// Start the Kademlia DHT in Server Mode
	kDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		panic(fmt.Errorf("failed to create DHT: %w", err))
	}
	if err = kDHT.Bootstrap(ctx); err != nil {
		panic(fmt.Errorf("failed to bootstrap DHT: %w", err))
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
		panic(err)
	}
}

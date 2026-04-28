package network

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
)

// LoadOrGenerateIdentity returns the persistent Ed25519 private key at
// keyPath, creating one on first boot. Shared by both binaries:
// internal/network's mode-suffixed boss/worker/api identities, and the
// dedicated relay binary's relay_identity.key. Stable peer IDs across
// restarts let other nodes cache our address and skip DHT lookups, and
// private swarms often allowlist peers by ID.
//
// A read failure is fine on the very first boot, but a successful read
// that fails to unmarshal means the file on disk is corrupt. We surface
// that so the operator realises their peer ID is about to change instead
// of silently regenerating.
func LoadOrGenerateIdentity(keyPath string) (crypto.PrivKey, error) {
	if keyBytes, err := os.ReadFile(keyPath); err == nil {
		priv, err := crypto.UnmarshalPrivateKey(keyBytes)
		if err == nil {
			return priv, nil
		}
		slog.Warn("corrupt identity file; regenerating",
			slog.String("path", keyPath),
			slog.Any(obs.FieldErr, err),
		)
	}

	fmt.Println("🔑 Generating new permanent node identity...")
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, fmt.Errorf("generate identity key: %w", err)
	}

	keyBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal identity key: %w", err)
	}
	if err := os.WriteFile(keyPath, keyBytes, 0600); err != nil {
		// Not fatal. The node can still run with an ephemeral key, but the
		// operator needs to know the peer ID won't be stable across restarts.
		slog.Warn("could not persist identity; peer ID will change on restart",
			slog.String("path", keyPath),
			slog.Any(obs.FieldErr, err),
		)
	}

	return priv, nil
}

// loadOrGenerateIdentity preserves the legacy package-internal call sites
// (createHost) that pass a mode string and want the .agentfm_<mode>_identity.key
// naming convention. New callers should use LoadOrGenerateIdentity directly.
func loadOrGenerateIdentity(mode string) (crypto.PrivKey, error) {
	return LoadOrGenerateIdentity(fmt.Sprintf(".agentfm_%s_identity.key", mode))
}

// createHost assembles the libp2p Host with the correct options for this
// role: PSK for private swarms, circuit relay in the right direction,
// NAT port mapping, and the AutoNAT reachability probe for non-relay
// nodes.
func createHost(cfg Config, bootstrapAddr string) (host.Host, error) {
	// Listen on both v4 and v6 to match the relay binary's dual-stack
	// config. A v6-only home network would otherwise be unreachable.
	listenAddrs := []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.ListenPort),
		fmt.Sprintf("/ip6/::/tcp/%d", cfg.ListenPort),
	}

	privKey, err := loadOrGenerateIdentity(cfg.Mode)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	options := []libp2p.Option{
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.NATPortMap(),
		libp2p.Identity(privKey), // Attach permanent identity to libp2p
	}

	if cfg.SwarmKeyPath != "" {
		psk, err := LoadSwarmKey(cfg.SwarmKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load swarm key: %w", err)
		}
		options = append(options, libp2p.PrivateNetwork(psk))
		fmt.Printf("🔒 Joining Private Swarm (Port: %d)\n", cfg.ListenPort)
	} else {
		fmt.Println("🌐 Joining Public P2P Mesh...")
	}

	if cfg.Mode == "relay" {
		fmt.Println("📡 Enabling Circuit Relay v2 Service (Anchor Node)")
		options = append(options, libp2p.EnableRelayService())
	} else {
		options = append(options, libp2p.EnableRelay(), libp2p.EnableHolePunching())

		if bootstrapAddr != "" {
			if relayInfo, err := parseRelayInfo(bootstrapAddr); err == nil {
				options = append(options, libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}))
			}
		}
	}

	h, err := libp2p.New(options...)
	if err != nil {
		return nil, err
	}

	if cfg.Mode != "relay" {
		if _, err = autonat.New(h); err != nil {
			slog.Warn("autonat unavailable; reachability autodetection disabled",
				slog.Any(obs.FieldErr, err),
			)
		} else {
			fmt.Println("🌐 [NAT] AutoNAT service started. Testing public reachability...")
		}
	}

	return h, nil
}

// initRoutingAndPubSub brings up the GossipSub topic mesh and the
// Kademlia DHT. Workers run the DHT as a client (they just want to be
// discoverable) while Boss and Relay nodes run it in server mode so they
// help route other peers.
func initRoutingAndPubSub(ctx context.Context, h host.Host, isWorker bool) (*pubsub.PubSub, *dht.IpfsDHT, error) {
	ps, err := pubsub.NewGossipSub(ctx, h, pubsub.WithFloodPublish(true))
	if err != nil {
		return nil, nil, err
	}
	var dhtOptions []dht.Option
	if isWorker {
		dhtOptions = append(dhtOptions, dht.Mode(dht.ModeClient))
	} else {
		dhtOptions = append(dhtOptions, dht.Mode(dht.ModeServer))
	}
	kademliaDHT, err := dht.New(ctx, h, dhtOptions...)
	if err != nil {
		return nil, nil, err
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, nil, err
	}
	return ps, kademliaDHT, nil
}

package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// --- loadOrGenerateIdentity ------------------------------------------------

// TestLoadOrGenerateIdentity_NewKey verifies the cold-start path: no key on
// disk, function mints a fresh Ed25519 key, persists it with mode 0600, and
// the persisted bytes round-trip cleanly through UnmarshalPrivateKey.
func TestLoadOrGenerateIdentity_NewKey(t *testing.T) {
	t.Chdir(t.TempDir())

	priv, err := loadOrGenerateIdentity("test_new")
	if err != nil {
		t.Fatalf("loadOrGenerateIdentity: %v", err)
	}
	if priv == nil {
		t.Fatal("nil priv")
	}
	if priv.Type() != crypto.Ed25519 {
		t.Errorf("key type = %v, want Ed25519", priv.Type())
	}

	keyPath := ".agentfm_test_new_identity.key"
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 0600", perm)
	}

	// Round-trip: the persisted bytes must deserialize back to the same key.
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	restored, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		t.Fatalf("unmarshal persisted: %v", err)
	}
	if !priv.Equals(restored) {
		t.Error("persisted key does not round-trip")
	}
}

// TestLoadOrGenerateIdentity_LoadsExisting guards against the "mints a fresh
// key on every restart" regression. Two calls with the same mode must return
// the same peer identity, because callers rely on this for stable peer IDs.
func TestLoadOrGenerateIdentity_LoadsExisting(t *testing.T) {
	t.Chdir(t.TempDir())

	priv1, err := loadOrGenerateIdentity("test_stable")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	priv2, err := loadOrGenerateIdentity("test_stable")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !priv1.Equals(priv2) {
		t.Error("second call generated a new key; expected to load the existing one")
	}

	// Also verify the derived peer ID is stable, which is what the mesh sees.
	id1, _ := peer.IDFromPrivateKey(priv1)
	id2, _ := peer.IDFromPrivateKey(priv2)
	if id1 != id2 {
		t.Errorf("peer IDs differ: %s vs %s", id1, id2)
	}
}

// TestLoadOrGenerateIdentity_CorruptedFile documents the current behaviour:
// if the key file exists but is garbage, the function silently ignores it
// and mints a fresh key. This is intentional so a corrupted key doesn't
// brick a Worker, but we assert the behaviour so any future change is
// explicit.
func TestLoadOrGenerateIdentity_CorruptedFile(t *testing.T) {
	t.Chdir(t.TempDir())

	keyPath := ".agentfm_test_corrupt_identity.key"
	if err := os.WriteFile(keyPath, []byte("not a valid libp2p key"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	priv, err := loadOrGenerateIdentity("test_corrupt")
	if err != nil {
		t.Fatalf("loadOrGenerateIdentity: %v", err)
	}
	if priv == nil {
		t.Fatal("expected a fresh key, got nil")
	}
	// The fresh key must be persisted, overwriting the garbage.
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read after regen: %v", err)
	}
	if _, err := crypto.UnmarshalPrivateKey(raw); err != nil {
		t.Errorf("regenerated key is not unmarshalable: %v", err)
	}
}

// TestLoadOrGenerateIdentity_UnpersistablePath exercises the H2-compliance
// fix where we stopped silently swallowing os.WriteFile errors. The function
// must still return a valid key (so the node can run ephemerally) but the
// failure is logged to stderr/stdout via fmt.Printf.
func TestLoadOrGenerateIdentity_UnpersistablePath(t *testing.T) {
	// Put a FILE at the location where the function expects to create one.
	// The subsequent WriteFile call can still succeed by truncating — so
	// instead, create a directory at the exact key path, which makes
	// os.WriteFile fail with "is a directory".
	tmp := t.TempDir()
	t.Chdir(tmp)

	keyPath := ".agentfm_test_unpersist_identity.key"
	if err := os.Mkdir(filepath.Join(tmp, keyPath), 0o755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}

	priv, err := loadOrGenerateIdentity("test_unpersist")
	if err != nil {
		t.Fatalf("function must still return a valid key, got error: %v", err)
	}
	if priv == nil {
		t.Fatal("nil priv on unpersistable path")
	}
	if priv.Type() != crypto.Ed25519 {
		t.Errorf("key type = %v, want Ed25519", priv.Type())
	}
}

// --- parseRelayInfo --------------------------------------------------------

// TestParseRelayInfo covers the two axes of the public input: correctly
// formed multiaddrs with a /p2p/<id> suffix (valid), and every way an
// operator might mistype the address string (invalid).
func TestParseRelayInfo(t *testing.T) {
	t.Parallel()

	// Deterministic peer ID literal, so we can assert on the parsed result.
	const validPeerID = "12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT"

	validCases := []struct {
		name   string
		addr   string
		wantIP string // substring we expect to see in at least one parsed addr
	}{
		{
			name:   "ip4 + tcp + p2p",
			addr:   fmt.Sprintf("/ip4/198.51.100.23/tcp/4001/p2p/%s", validPeerID),
			wantIP: "198.51.100.23",
		},
		{
			name:   "ip6 + tcp + p2p",
			addr:   fmt.Sprintf("/ip6/2a01:4f8:1c19:a81c::1/tcp/4001/p2p/%s", validPeerID),
			wantIP: "2a01:4f8:1c19:a81c::1",
		},
		{
			name:   "loopback",
			addr:   fmt.Sprintf("/ip4/127.0.0.1/tcp/4001/p2p/%s", validPeerID),
			wantIP: "127.0.0.1",
		},
	}

	for _, tc := range validCases {
		tc := tc
		t.Run("valid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			info, err := parseRelayInfo(tc.addr)
			if err != nil {
				t.Fatalf("parseRelayInfo: %v", err)
			}
			if info.ID.String() != validPeerID {
				t.Errorf("peer id = %s, want %s", info.ID.String(), validPeerID)
			}
			if len(info.Addrs) == 0 {
				t.Fatal("no addrs parsed")
			}
			if !strings.Contains(info.Addrs[0].String(), tc.wantIP) {
				t.Errorf("first addr %s does not contain %s", info.Addrs[0], tc.wantIP)
			}
		})
	}

	invalidCases := []struct {
		name       string
		addr       string
		wantSubstr string
	}{
		{
			name:       "empty string",
			addr:       "",
			wantSubstr: "invalid relay multiaddr",
		},
		{
			name:       "random junk",
			addr:       "not-a-multiaddr",
			wantSubstr: "invalid relay multiaddr",
		},
		{
			name: "missing /p2p/ component",
			addr: "/ip4/127.0.0.1/tcp/4001",
			// multiaddr parses fine; AddrInfoFromP2pAddr returns an error.
			wantSubstr: "",
		},
		{
			name: "malformed peer id",
			addr: "/ip4/127.0.0.1/tcp/4001/p2p/not-a-real-id",
			// multiaddr rejects this at the parsing stage.
			wantSubstr: "invalid relay multiaddr",
		},
	}

	for _, tc := range invalidCases {
		tc := tc
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseRelayInfo(tc.addr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// --- mdnsNotifee.HandlePeerFound -------------------------------------------

// TestMDNSNotifee_HandlePeerFound_Dials verifies the mdns callback actually
// establishes a connection. The hosts are linked (can connect) but not yet
// connected, so any pre-existing connection would be a test setup bug.
func TestMDNSNotifee_HandlePeerFound_Dials(t *testing.T) {
	hosts := testutil.NewLinkedMesh(t, 2)
	a, b := hosts[0], hosts[1]

	// Precondition: not connected.
	if a.Network().Connectedness(b.ID()) == netcore.Connected {
		t.Fatal("precondition failed: hosts already connected")
	}

	notifee := &mdnsNotifee{h: a}
	notifee.HandlePeerFound(peer.AddrInfo{
		ID:    b.ID(),
		Addrs: b.Addrs(),
	})

	// The dial runs synchronously inside HandlePeerFound, so by the time it
	// returns the connection state should be Connected. Use testutil.Eventually() as
	// a paranoid guard against mocknet's internal goroutine scheduling.
	testutil.Eventually(t, 2*time.Second, func() bool {
		return a.Network().Connectedness(b.ID()) == netcore.Connected
	}, "peer A to be connected to peer B")
}

// TestMDNSNotifee_HandlePeerFound_UnreachablePeer exercises the recently
// added error-logging branch. The callback must not panic when the dial
// fails; it should just log and return.
func TestMDNSNotifee_HandlePeerFound_UnreachablePeer(t *testing.T) {
	hosts := testutil.NewLinkedMesh(t, 1)
	notifee := &mdnsNotifee{h: hosts[0]}

	// Invent a peer ID that isn't in the mesh. Dial must fail gracefully.
	bogusID, err := peer.Decode("12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT")
	if err != nil {
		t.Fatalf("peer.Decode: %v", err)
	}

	// The function returns nothing but must not panic. If we reach the line
	// after the call, the test passes.
	notifee.HandlePeerFound(peer.AddrInfo{ID: bogusID})
}

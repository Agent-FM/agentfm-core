package boss

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

// relayAddrDisallowed reports whether a candidate relay multiaddr points
// at an internal address the boss must not be tricked into dialing (an
// SSRF port-scan oracle when the API is exposed off-host). Loopback is
// allowed — it is the desktop's own local relay — as is any global
// unicast address (real VPS relays). Private (RFC1918/ULA), link-local,
// unspecified and multicast ranges are refused. A /dns-based addr has no
// literal IP here and is left to the bounded dial.
func relayAddrDisallowed(maddr multiaddr.Multiaddr) bool {
	var ipStr string
	if v, err := maddr.ValueForProtocol(multiaddr.P_IP4); err == nil {
		ipStr = v
	} else if v, err := maddr.ValueForProtocol(multiaddr.P_IP6); err == nil {
		ipStr = v
	}
	if ipStr == "" {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return false
	}
	return ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// relayTestRequest is the JSON body for POST /api/relay/test.
type relayTestRequest struct {
	Multiaddr string `json:"multiaddr"`
}

// relayTestResponse is the JSON response for POST /api/relay/test.
type relayTestResponse struct {
	OK     bool   `json:"ok"`
	PeerID string `json:"peer_id,omitempty"`
	Error  string `json:"error,omitempty"`
}

// handleRelayTest probes a candidate relay multiaddr from the running
// boss. The desktop "Test connection" button hits this before a user
// commits a project: it parses the multiaddr, attempts a bounded dial,
// and returns whether the relay answered.
//
// The dial uses the same libp2p host the boss already runs on, which
// means a successful response also primes the peerstore — making the
// subsequent backend restart (with -bootstrap <multiaddr>) hit a warm
// cache instead of re-discovering from scratch.
func (b *Boss) handleRelayTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, relayTestResponse{
			OK:    false,
			Error: "POST only",
		})
		return
	}

	// Bound the request body — the multiaddr field is short, but a
	// malicious / accidental large body should not cost us memory.
	var req relayTestRequest
	limited := io.LimitReader(r.Body, 4*1024)
	if err := json.NewDecoder(limited).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, relayTestResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	addr := strings.TrimSpace(req.Multiaddr)
	if addr == "" {
		writeJSON(w, http.StatusBadRequest, relayTestResponse{
			OK:    false,
			Error: "field 'multiaddr' is required",
		})
		return
	}

	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		writeJSON(w, http.StatusOK, relayTestResponse{
			OK:    false,
			Error: "invalid multiaddr: " + err.Error(),
		})
		return
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		writeJSON(w, http.StatusOK, relayTestResponse{
			OK:    false,
			Error: "multiaddr missing /p2p/<peer> suffix: " + err.Error(),
		})
		return
	}

	if relayAddrDisallowed(maddr) {
		writeJSON(w, http.StatusOK, relayTestResponse{
			OK:    false,
			Error: "relay address is in a disallowed (private/link-local) range",
		})
		return
	}

	if b.node == nil || b.node.Host == nil {
		writeJSON(w, http.StatusServiceUnavailable, relayTestResponse{
			OK:    false,
			Error: "boss host not initialized",
		})
		return
	}

	if b.node.Host.Network().Connectedness(info.ID) == network.Connected {
		writeJSON(w, http.StatusOK, relayTestResponse{
			OK:     true,
			PeerID: info.ID.String(),
		})
		return
	}

	b.node.Host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)

	dialCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := b.node.Host.Connect(dialCtx, *info); err != nil {
		writeJSON(w, http.StatusOK, relayTestResponse{
			OK:     false,
			PeerID: info.ID.String(),
			Error:  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, relayTestResponse{
		OK:     true,
		PeerID: info.ID.String(),
	})
}

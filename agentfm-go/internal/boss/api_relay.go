package boss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

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

	if b.node == nil || b.node.Host == nil {
		writeJSON(w, http.StatusServiceUnavailable, relayTestResponse{
			OK:    false,
			Error: "boss host not initialized",
		})
		return
	}

	// Bounded dial. 5s is enough for a healthy lighthouse on the open
	// internet; anything slower is effectively unreachable from a
	// desktop boss anyway.
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

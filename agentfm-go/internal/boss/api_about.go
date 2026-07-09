package boss

import (
	"context"
	"net/http"
	"time"

	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// aboutResponse is the JSON payload for GET /v1/about.
type aboutResponse struct {
	BossPeerID      string  `json:"boss_peer_id"`
	RelayPeerID     string  `json:"relay_peer_id"`
	RelayMultiaddr  string  `json:"relay_multiaddr"`
	ReputationFloor float64 `json:"reputation_floor"`
	LedgerTreeSize  uint64  `json:"ledger_tree_size"`
	Version         string  `json:"version"`
	UptimeSeconds   int64   `json:"uptime_seconds"`
}

// handleAbout services GET /v1/about — returns a backend identity and
// status snapshot. Intended for the desktop app and tooling that need to
// verify which node they are talking to without a full API call.
func (b *Boss) handleAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var bossPeerID string
	if b.node != nil && b.node.Host != nil {
		bossPeerID = b.node.Host.ID().String()
	}

	// Relay fields: only populated when the relay peer is currently
	// connected. Empty fields signal "no relay" or "configured but
	// unreachable" — desktop UI surfaces this as "not connected to relay".
	var relayPeerID, relayMultiaddr string
	if b.node != nil && b.node.Host != nil && b.node.RelayPeerID != "" {
		if b.node.Host.Network().Connectedness(b.node.RelayPeerID) == netcore.Connected {
			relayPeerID = b.node.RelayPeerID.String()
			relayMultiaddr = b.node.RelayAddr
		}
	}

	// ledger_tree_size: pull from the live log head, nil-safe.
	var treeSize uint64
	if b.ledger != nil {
		if head, err := b.ledger.Head(context.Background()); err == nil && head != nil {
			treeSize = head.TreeSize
		}
	}

	var uptimeSecs int64
	if !b.startedAt.IsZero() {
		uptimeSecs = int64(time.Since(b.startedAt).Seconds())
	}

	writeJSON(w, http.StatusOK, aboutResponse{
		BossPeerID:      bossPeerID,
		RelayPeerID:     relayPeerID,
		RelayMultiaddr:  relayMultiaddr,
		ReputationFloor: b.reputationFloor,
		LedgerTreeSize:  treeSize,
		Version:         version.AppVersion,
		UptimeSeconds:   uptimeSecs,
	})
}

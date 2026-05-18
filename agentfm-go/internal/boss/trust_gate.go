package boss

import (
	"context"
	"errors"

	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	ErrPeerIsEquivocator    = errors.New("peer_is_equivocator")
	ErrReputationBelowFloor = errors.New("reputation_below_floor")
)

// TrustOutcome is the result of the two-check dispatch gate.
type TrustOutcome struct {
	Allowed bool
	Reason  string
	Score   float64
}

// checkTrust is the simple two-check dispatch gate for v1.3.1.
//
//  1. Equivocator check — hard floor, no override.
//  2. Reputation floor — soft, configurable via --reputation-floor.
//
// Returns Allowed=true only if both pass.
func (b *Boss) checkTrust(ctx context.Context, w types.WorkerProfile) TrustOutcome {
	if b.ledger != nil {
		pid, err := peer.Decode(w.PeerID)
		if err == nil {
			marked, _ := b.ledger.IsEquivocator(ctx, []byte(pid))
			if marked {
				return TrustOutcome{Allowed: false, Reason: ErrPeerIsEquivocator.Error()}
			}
		}
	}
	if b.reputationEngine != nil {
		s := b.reputationEngine.Score(w.PeerID)
		if s < b.reputationFloor {
			return TrustOutcome{Allowed: false, Reason: ErrReputationBelowFloor.Error(), Score: s}
		}
	}
	return TrustOutcome{Allowed: true}
}

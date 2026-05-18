package integration

import (
	"testing"
	"time"

	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// mintKeyP2 generates a fresh Ed25519 private key. Used by tests that
// need an identity that is not tied to any running libp2p host.
func mintKeyP2(t *testing.T) crypto.PrivKey {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	return priv
}

// freshSimpleRating returns a minimum-viable Rating envelope authored
// by peerID. Used by tests that only need a valid SignedEntry payload.
func freshSimpleRating(peerID peer.ID) *pb.SignedEntry {
	return &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(peerID),
		SubjectPeerId:   make([]byte, 32),
		Dimension:       "honesty",
		Score:           0.5,
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
}

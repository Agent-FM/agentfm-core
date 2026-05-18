package ledger_test

import (
	"context"
	"path/filepath"
	"testing"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// freshKey returns a brand-new Ed25519 private key for tests.
// Mirrors how worker_identity.key is generated in production.
func freshKey(t *testing.T) crypto.PrivKey {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return priv
}

// New() must succeed in local-only mode (nil PubSub). Asserting the
// shape of the returned Ledger pins the public contract; the per-method
// behaviour is covered by the impl tests in impl_test.go.
func TestNew_LocalOnly_Succeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	l, err := ledger.New(path, freshKey(t), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l == nil {
		t.Fatal("New returned nil Ledger with nil error")
	}
	t.Cleanup(func() { _ = l.Close() })

	// Head on a fresh ledger is nil, nil.
	h, err := l.Head(context.Background())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if h != nil {
		t.Fatalf("fresh Ledger Head() returned %+v, want nil", h)
	}
}

// The Ledger interface contract must be exported so other packages can
// type-assert / mock against it. Compile-time check — fails compilation
// if the interface shape changes incompatibly.
type _ledgerContractCheck = interface {
	Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error)
	Head(ctx context.Context) (*pb.LogHead, error)
	Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error)
	VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error
	InboxHas(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error)
	IsEquivocator(ctx context.Context, peerID []byte) (bool, error)
	AcceptEntry(ctx context.Context, payload []byte) error
	LastInboxIdx(ctx context.Context) (uint64, error)
	Close() error
}

var _ _ledgerContractCheck = (ledger.Ledger)(nil)

// New with nil key fails — bootstrap contract check.
func TestNew_NilKey_Fails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	if _, err := ledger.New(path, nil, nil); err == nil {
		t.Fatal("expected error for nil key, got nil")
	}
}

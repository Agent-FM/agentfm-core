package ledger_test

import (
	"context"
	"errors"
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

// New() must return ErrNotImplemented while the package is still in its
// P0-3 stub state. The day P1-1..P1-4 land and New() starts returning a
// real Ledger, this test will (deliberately) flip to failing — replace
// it then with a constructor-success test.
func TestNew_ReturnsErrNotImplemented(t *testing.T) {
	got, err := ledger.New(t.TempDir()+"/ledger.db", freshKey(t))
	if !errors.Is(err, ledger.ErrNotImplemented) {
		t.Fatalf("want ErrNotImplemented, got err=%v", err)
	}
	if got != nil {
		t.Fatalf("want nil Ledger while stubbed, got %#v", got)
	}
}

// The Ledger interface contract must be exported so other packages can
// type-assert / mock against it. A compile-time check is enough — the
// var below fails to compile if the interface shape changes
// incompatibly.
type _ledgerContractCheck = interface {
	Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error)
	Head(ctx context.Context) (*pb.LogHead, error)
	Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error)
	VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error
	Close() error
}

var _ _ledgerContractCheck = (ledger.Ledger)(nil)

package ledger_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// signWithIdent constructs (priv, raterIDBytes) tied together.
func signWithIdent(t *testing.T) (crypto.PrivKey, []byte) {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("peer id: %v", err)
	}
	return priv, []byte(id)
}

func ratingFor(raterID []byte) *pb.SignedEntry {
	return &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     raterID,
		SubjectPeerId:   make([]byte, 32),
		Dimension:       "honesty",
		Score:           0.5,
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
}

func TestProve_NoEntries_ReturnsErrEntryNotInLog(t *testing.T) {
	priv, _ := signWithIdent(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "p.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	var unknown [32]byte
	unknown[0] = 0xff
	_, err = l.Prove(context.Background(), unknown)
	if !errors.Is(err, ledger.ErrEntryNotInLog) {
		t.Fatalf("want ErrEntryNotInLog, got %v", err)
	}
}

func TestProve_SingleEntry_VerifyInclusionProofPasses(t *testing.T) {
	priv, rid := signWithIdent(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "p.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	h, err := l.Append(context.Background(), ratingFor(rid))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	proof, err := l.Prove(context.Background(), h)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if proof.Position != 0 {
		t.Fatalf("Position = %d, want 0", proof.Position)
	}
	if proof.LogHead == nil {
		t.Fatal("LogHead nil")
	}
	if proof.LogHead.TreeSize != 1 {
		t.Fatalf("LogHead.TreeSize = %d, want 1", proof.LogHead.TreeSize)
	}
	ok, err := ledger.VerifyInclusionProof(proof)
	if err != nil {
		t.Fatalf("VerifyInclusionProof: %v", err)
	}
	if !ok {
		t.Fatal("VerifyInclusionProof returned false on honest proof")
	}
}

func TestProve_MultipleEntries_AllVerify(t *testing.T) {
	priv, rid := signWithIdent(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "p.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	hashes := make([][32]byte, 10)
	for i := range hashes {
		h, err := l.Append(context.Background(), ratingFor(rid))
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		hashes[i] = h
	}
	for i, h := range hashes {
		proof, err := l.Prove(context.Background(), h)
		if err != nil {
			t.Fatalf("Prove idx=%d: %v", i, err)
		}
		if proof.Position != uint64(i) {
			t.Errorf("Position = %d, want %d", proof.Position, i)
		}
		ok, err := ledger.VerifyInclusionProof(proof)
		if err != nil {
			t.Fatalf("Verify idx=%d: %v", i, err)
		}
		if !ok {
			t.Fatalf("Verify failed at idx=%d", i)
		}
	}
}

func TestVerifyInclusionProof_TamperedRoot_Fails(t *testing.T) {
	priv, rid := signWithIdent(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "p.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	h, err := l.Append(context.Background(), ratingFor(rid))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	proof, err := l.Prove(context.Background(), h)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	proof.LogHead.RootHash[0] ^= 0x01
	ok, _ := ledger.VerifyInclusionProof(proof)
	if ok {
		t.Fatal("Verify accepted proof with tampered root")
	}
}

func TestVerifyInclusionProof_TamperedEntry_Fails(t *testing.T) {
	priv, rid := signWithIdent(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "p.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	h, err := l.Append(context.Background(), ratingFor(rid))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	proof, err := l.Prove(context.Background(), h)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	// InclusionProof.Entry is now a SignedEntry wrapper (Fix-6).
	if r := proof.Entry.GetRating(); r != nil {
		r.Score = -0.99
	}
	ok, _ := ledger.VerifyInclusionProof(proof)
	if ok {
		t.Fatal("Verify accepted proof with tampered entry")
	}
}

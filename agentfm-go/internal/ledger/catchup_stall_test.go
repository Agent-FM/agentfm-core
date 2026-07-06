package ledger_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	libnet "github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/proto"
)

const headFetchProtocolForTest = "/agentfm/head-fetch/1.0.0"

// TestCatchUp_RejectsStalledCursor proves the own-log catch-up loop
// terminates when a malicious relay signs a head with an enormous
// TreeSize and then serves full pages whose entry idx never advances.
// Without the non-advancing guard, the boss startup goroutine livelocks.
func TestCatchUp_RejectsStalledCursor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	srcKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen src key: %v", err)
	}
	srcHost := testutil.NewHostWithKey(t, srcKey)
	tgtHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, srcHost, tgtHost)

	root := make([]byte, 32)
	root[0] = 0x01
	head := &pb.LogHead{
		PeerId:          []byte(srcHost.ID()),
		TreeSize:        1 << 40,
		RootHash:        root,
		TimestampUnixNs: time.Now().UnixNano(),
	}
	canonical, err := pb.CanonicalLogHead(head)
	if err != nil {
		t.Fatalf("canonical head: %v", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := srcKey.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign head: %v", err)
	}
	head.Signature = sig
	headBytes, err := proto.Marshal(head)
	if err != nil {
		t.Fatalf("marshal head: %v", err)
	}

	srcHost.SetStreamHandler(headFetchProtocolForTest, func(s libnet.Stream) {
		defer func() { _ = s.Close() }()
		_ = s.SetDeadline(time.Now().Add(30 * time.Second))
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(headBytes)))
		if _, err := s.Write(lenBuf[:]); err != nil {
			return
		}
		_, _ = s.Write(headBytes)
	})

	const page = 1000
	srcHost.SetStreamHandler(network.LedgerFetchProtocol, func(s libnet.Stream) {
		defer func() { _ = s.Close() }()
		_ = s.SetDeadline(time.Now().Add(30 * time.Second))
		var req [16]byte
		if _, err := io.ReadFull(s, req[:]); err != nil {
			return
		}
		var cb [8]byte
		binary.BigEndian.PutUint64(cb[:], page)
		if _, err := s.Write(cb[:]); err != nil {
			return
		}
		for i := 0; i < page; i++ {
			var eh [12]byte
			binary.BigEndian.PutUint64(eh[:8], 1)
			binary.BigEndian.PutUint32(eh[8:], 0)
			if _, err := s.Write(eh[:]); err != nil {
				return
			}
		}
	})

	tgtKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen tgt key: %v", err)
	}
	tgtLedger, err := ledger.NewWithOptions(filepath.Join(t.TempDir(), "tgt.db"), tgtKey, nil, ledger.Options{Host: tgtHost})
	if err != nil {
		t.Fatalf("open tgt: %v", err)
	}
	t.Cleanup(func() { _ = tgtLedger.Close() })

	start := time.Now()
	err = ledger.CatchUp(ctx, tgtLedger, tgtHost, srcHost.ID())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected CatchUp to reject a non-advancing full page, got nil")
	}
	if !strings.Contains(err.Error(), "advance") && !strings.Contains(err.Error(), "protocol violation") {
		t.Fatalf("expected non-advancing guard error, got: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("CatchUp did not terminate promptly (%.1fs) — livelock guard missing", elapsed.Seconds())
	}
}

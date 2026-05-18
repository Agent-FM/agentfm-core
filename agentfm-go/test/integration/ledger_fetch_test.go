package integration

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// TestLedgerFetch_ClientPullsServerEntries spins two libp2p hosts:
// server S has a ledger with 5 entries; client C opens
// LedgerFetchProtocol against S, pulls all 5, and verifies each
// entry's signature via ledger.VerifyEntry. The fetch protocol is
// P2-5's pull-on-demand mechanism for inclusion-proof construction.
func TestLedgerFetch_ClientPullsServerEntries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 2)
	serverHost, clientHost := hosts[0], hosts[1]

	serverKey := mintKeyP2(t)
	// We need the server's libp2p host to match serverKey so the
	// ledger's PID derivation aligns with the libp2p identity. Use a
	// dedicated host with the key.
	srvHost := testutil.NewHostWithKey(t, serverKey)
	testutil.ConnectHosts(t, srvHost, clientHost)
	_ = serverHost // not used; the original mesh hosts are auxiliary.

	psSrv, err := pubsub.NewGossipSub(ctx, srvHost, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub: %v", err)
	}

	srvLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "srv.db"),
		serverKey,
		psSrv,
		ledger.Options{Host: srvHost},
	)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	t.Cleanup(func() { _ = srvLedger.Close() })

	srvPID, err := peer.IDFromPrivateKey(serverKey)
	if err != nil {
		t.Fatalf("server pid: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := srvLedger.Append(ctx, freshSimpleRating(srvPID)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// Client pulls entries 1..5 (1-based).
	fetched, err := ledger.FetchClient(ctx, clientHost, srvPID, 1, 5)
	if err != nil {
		t.Fatalf("FetchClient: %v", err)
	}
	if len(fetched) != 5 {
		t.Fatalf("fetched %d entries, want 5", len(fetched))
	}
	for i, fe := range fetched {
		if fe.Idx != uint64(i+1) {
			t.Errorf("fetched[%d].Idx = %d, want %d", i, fe.Idx, i+1)
		}
		var signed pb.SignedEntry
		if err := proto.Unmarshal(fe.Payload, &signed); err != nil {
			t.Errorf("unmarshal idx=%d: %v", fe.Idx, err)
			continue
		}
		ok, err := ledger.VerifyEntry(&signed)
		if err != nil {
			t.Errorf("VerifyEntry idx=%d: %v", fe.Idx, err)
			continue
		}
		if !ok {
			t.Errorf("VerifyEntry idx=%d returned false", fe.Idx)
		}
	}
}

// TestLedgerFetch_PartialRangeRespectsCount asks for fewer entries
// than the server has and confirms only that many come back.
func TestLedgerFetch_PartialRangeRespectsCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 1)
	clientHost := hosts[0]

	serverKey := mintKeyP2(t)
	srvHost := testutil.NewHostWithKey(t, serverKey)
	testutil.ConnectHosts(t, srvHost, clientHost)

	psSrv, err := pubsub.NewGossipSub(ctx, srvHost, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub: %v", err)
	}
	srvLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "srv.db"),
		serverKey,
		psSrv,
		ledger.Options{Host: srvHost},
	)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	t.Cleanup(func() { _ = srvLedger.Close() })

	srvPID, err := peer.IDFromPrivateKey(serverKey)
	if err != nil {
		t.Fatalf("server pid: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := srvLedger.Append(ctx, freshSimpleRating(srvPID)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	fetched, err := ledger.FetchClient(ctx, clientHost, srvPID, 3, 4)
	if err != nil {
		t.Fatalf("FetchClient: %v", err)
	}
	if len(fetched) != 4 {
		t.Fatalf("fetched %d, want 4", len(fetched))
	}
	if fetched[0].Idx != 3 {
		t.Errorf("first idx = %d, want 3", fetched[0].Idx)
	}
	if fetched[3].Idx != 6 {
		t.Errorf("last idx = %d, want 6", fetched[3].Idx)
	}
	for _, fe := range fetched {
		if len(fe.Payload) == 0 {
			t.Errorf("idx=%d payload empty", fe.Idx)
		}
		if bytes.Equal(fe.Payload, []byte{0}) {
			t.Errorf("idx=%d payload looks zeroed", fe.Idx)
		}
	}
}

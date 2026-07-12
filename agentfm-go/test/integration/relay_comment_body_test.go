package integration

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestRelay_StoresAndServesCommentBodies(t *testing.T) {
	tmp := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	relayKey := mintEdKey(t)
	bossKey := mintEdKey(t)

	relayHost := testutil.NewHostWithKey(t, relayKey)
	bossHost := testutil.NewHostWithKey(t, bossKey)
	testutil.ConnectHosts(t, relayHost, bossHost)

	bossPeerID, err := peer.IDFromPrivateKey(bossKey)
	if err != nil {
		t.Fatalf("boss peer id: %v", err)
	}

	relayPS, bossPS := newPubSubPair(t, ctx, relayHost, bossHost)

	bossStore, err := comments.Open(filepath.Join(tmp, "boss_comments"))
	if err != nil {
		t.Fatalf("open boss comments store: %v", err)
	}
	bossSrv := comments.NewServer(bossHost, bossStore)
	bossSrv.Start()
	defer bossSrv.Stop()

	body := []byte("solid worker, fast artifact turnaround")
	cid, err := bossStore.Put(body)
	if err != nil {
		t.Fatalf("put body on boss: %v", err)
	}

	relayStore, err := comments.Open(filepath.Join(tmp, "relay_comments"))
	if err != nil {
		t.Fatalf("open relay comments store: %v", err)
	}
	relaySrv := comments.NewServer(relayHost, relayStore)
	relaySrv.Start()
	defer relaySrv.Stop()

	arch, err := ledger.NewWithOptions(filepath.Join(tmp, "relay_ledger.db"), relayKey, relayPS,
		ledger.Options{Host: relayHost, Comments: relayStore})
	if err != nil {
		t.Fatalf("open archive ledger: %v", err)
	}
	defer func() { _ = arch.Close() }()

	time.Sleep(800 * time.Millisecond)

	g, err := ledger.NewWithOptions(filepath.Join(tmp, "boss_ledger.db"), bossKey, bossPS,
		ledger.Options{Host: bossHost})
	if err != nil {
		t.Fatalf("open boss ledger: %v", err)
	}
	defer func() { _ = g.Close() }()

	comment := &pb.SignedEntry{Body: &pb.SignedEntry_Comment{Comment: &pb.Comment{
		RaterPeerId:     []byte(bossPeerID),
		SubjectPeerId:   bytes.Repeat([]byte{0xcd}, 32),
		Language:        "en",
		TextCid:         cid,
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	if _, err := g.Append(ctx, comment); err != nil {
		t.Fatalf("boss Append comment: %v", err)
	}

	testutil.Eventually(t, 10*time.Second, func() bool {
		return relayStore.Has(cid)
	}, "relay should fetch and store the comment body after ingesting the envelope")

	got, err := comments.Fetch(ctx, bossHost, relayHost.ID(), cid)
	if err != nil {
		t.Fatalf("fetch body from relay: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("relay served wrong body: got %q want %q", got, body)
	}
}

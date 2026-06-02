package ledger_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

func TestCatchUpInbox_PullsAndAcceptsThirdPartyEntries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 2)
	srcHost, tgtHost := hosts[0], hosts[1]

	srcKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen src key: %v", err)
	}
	tgtKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen tgt key: %v", err)
	}
	raterKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen rater key: %v", err)
	}
	raterID, err := peer.IDFromPrivateKey(raterKey)
	if err != nil {
		t.Fatalf("derive rater id: %v", err)
	}

	srcDB := filepath.Join(t.TempDir(), "src.db")
	srcLedger, err := ledger.NewWithOptions(srcDB, srcKey, nil, ledger.Options{Host: srcHost})
	if err != nil {
		t.Fatalf("open src ledger: %v", err)
	}
	t.Cleanup(func() { _ = srcLedger.Close() })

	tgtDB := filepath.Join(t.TempDir(), "tgt.db")
	tgtLedger, err := ledger.NewWithOptions(tgtDB, tgtKey, nil, ledger.Options{Host: tgtHost})
	if err != nil {
		t.Fatalf("open tgt ledger: %v", err)
	}
	t.Cleanup(func() { _ = tgtLedger.Close() })

	var prev [32]byte
	hashes := make([][32]byte, 0, 3)
	for i := 0; i < 3; i++ {
		entry := &pb.SignedEntry{
			Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
				RaterPeerId:     []byte(raterID),
				SubjectPeerId:   []byte("subject-peer-x"),
				Dimension:       "honesty",
				Score:           float64(i+1) / 10,
				TimestampUnixNs: time.Now().UnixNano(),
			}},
		}
		if err := ledger.SignEntry(raterKey, entry, prev); err != nil {
			t.Fatalf("sign %d: %v", i, err)
		}
		payload, err := proto.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal %d: %v", i, err)
		}
		h := ledger.EntryHash(entry)
		if err := srcLedger.Store().InsertInboxEntry(ctx, []byte(raterID), h, prev, payload); err != nil {
			t.Fatalf("seed inbox %d: %v", i, err)
		}
		hashes = append(hashes, h)
		prev = h
	}

	for _, h := range hashes {
		ok, err := tgtLedger.InboxHas(ctx, []byte(raterID), h)
		if err != nil {
			t.Fatalf("pre-check InboxHas: %v", err)
		}
		if ok {
			t.Fatalf("target unexpectedly already has entry %x", h[:8])
		}
	}

	if err := ledger.CatchUpInbox(ctx, tgtLedger, tgtHost, srcHost.ID()); err != nil {
		t.Fatalf("catch-up inbox: %v", err)
	}

	for i, h := range hashes {
		ok, err := tgtLedger.InboxHas(ctx, []byte(raterID), h)
		if err != nil {
			t.Fatalf("post-check InboxHas %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("target missing entry %d (hash %x) after CatchUpInbox", i, h[:8])
		}
	}
}

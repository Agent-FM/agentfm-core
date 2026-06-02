package ledger_test

import (
	"context"
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
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// maxInboxFetchEntriesForTest mirrors the package-internal
// maxInboxFetchEntries constant in inbox_fetch.go. MUST be kept in
// sync with the production constant.
const maxInboxFetchEntriesForTest = 1000

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

// TestCatchUpInbox_RejectsStalledCursor proves the loop terminates
// when a byzantine source returns a full page of entries whose
// rowids never advance past the requested cursor. Without the
// max-seen guard, the loop would re-request indefinitely.
func TestCatchUpInbox_RejectsStalledCursor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 2)
	srcHost, tgtHost := hosts[0], hosts[1]

	srcHost.SetStreamHandler(network.InboxFetchProtocol, func(s libnet.Stream) {
		defer func() { _ = s.Close() }()
		if err := s.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return
		}
		var hdr [16]byte
		if _, err := io.ReadFull(s, hdr[:]); err != nil {
			return
		}
		var cb [8]byte
		binary.BigEndian.PutUint64(cb[:], uint64(maxInboxFetchEntriesForTest))
		if _, err := s.Write(cb[:]); err != nil {
			return
		}
		for i := 0; i < maxInboxFetchEntriesForTest; i++ {
			var eh [12]byte
			binary.BigEndian.PutUint64(eh[:8], 1)
			binary.BigEndian.PutUint32(eh[8:], 1)
			if _, err := s.Write(eh[:]); err != nil {
				return
			}
			if _, err := s.Write([]byte{0x00}); err != nil {
				return
			}
		}
	})

	tgtKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen tgt key: %v", err)
	}
	tgtDB := filepath.Join(t.TempDir(), "tgt.db")
	tgtLedger, err := ledger.NewWithOptions(tgtDB, tgtKey, nil, ledger.Options{Host: tgtHost})
	if err != nil {
		t.Fatalf("open tgt: %v", err)
	}
	t.Cleanup(func() { _ = tgtLedger.Close() })

	err = ledger.CatchUpInbox(ctx, tgtLedger, tgtHost, srcHost.ID())
	if err == nil {
		t.Fatal("expected CatchUpInbox to return a protocol-violation error on stalled cursor")
	}
	if !strings.Contains(err.Error(), "rowids <= cursor") && !strings.Contains(err.Error(), "protocol violation") {
		t.Fatalf("expected protocol-violation error, got: %v", err)
	}
}

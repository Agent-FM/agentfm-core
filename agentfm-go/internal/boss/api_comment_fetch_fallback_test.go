package boss

import (
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
)

func newFallbackBoss(t *testing.T, opts func(*Boss)) (*Boss, *comments.Store) {
	t.Helper()
	localStore, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}
	b := &Boss{
		node:          &network.MeshNode{Host: testutil.NewHost(t)},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		commentsStore: localStore,
	}
	if opts != nil {
		opts(b)
	}
	return b, localStore
}

func TestCommentBodyGet_FetchesFromAuthorOnMiss(t *testing.T) {
	authorHost := testutil.NewHost(t)
	authorStore, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("comments.Open author: %v", err)
	}
	srv := comments.NewServer(authorHost, authorStore)
	srv.Start()
	defer srv.Stop()

	body := []byte("fetched over p2p from the author")
	cid, err := authorStore.Put(body)
	if err != nil {
		t.Fatalf("author Put: %v", err)
	}

	subject := testutil.NewHost(t).ID()
	readStore := testutil.OpenTestStore(t)
	testutil.AppendInboxComment(t, readStore, authorHost, subject, cid)

	b, localStore := newFallbackBoss(t, func(b *Boss) { b.readStore = readStore })
	testutil.ConnectHosts(t, b.node.Host, authorHost)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/v1/peers/%s/comments/%s", subject.String(), hex.EncodeToString(cid)), nil)
	b.handlePeersForTest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != string(body) {
		t.Fatalf("body mismatch: %q", got)
	}
	if !localStore.Has(cid) {
		t.Fatalf("fetched body was not cached in the local store")
	}
}

func TestCommentBodyGet_FetchesFromRelayWhenAuthorUnknown(t *testing.T) {
	relayHost := testutil.NewHost(t)
	relayStore, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("comments.Open relay: %v", err)
	}
	srv := comments.NewServer(relayHost, relayStore)
	srv.Start()
	defer srv.Stop()

	body := []byte("fetched over p2p from the relay archive")
	cid, err := relayStore.Put(body)
	if err != nil {
		t.Fatalf("relay Put: %v", err)
	}

	b, _ := newFallbackBoss(t, func(b *Boss) { b.node.RelayPeerID = relayHost.ID() })
	testutil.ConnectHosts(t, b.node.Host, relayHost)

	subject := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/v1/peers/%s/comments/%s.json", subject.String(), hex.EncodeToString(cid)), nil)
	b.handlePeersForTest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCommentBodyGet_400OnStructurallyInvalidCID(t *testing.T) {
	relayHost := testutil.NewHost(t)
	b, _ := newFallbackBoss(t, func(b *Boss) { b.node.RelayPeerID = relayHost.ID() })

	subject := testutil.NewHost(t).ID()
	for _, cid := range []string{"ab", "12ab34", "ff" + hex.EncodeToString(make([]byte, 33))} {
		start := time.Now()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET",
			fmt.Sprintf("/v1/peers/%s/comments/%s", subject.String(), cid), nil)
		b.handlePeersForTest(rec, req)

		if rec.Code != 400 {
			t.Fatalf("cid %q: status=%d; want 400", cid, rec.Code)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("cid %q: malformed CID took %v; must reject without remote fetches", cid, elapsed)
		}
	}
}

func TestCommentBodyGet_404WhenNoSourceHasIt(t *testing.T) {
	b, _ := newFallbackBoss(t, nil)

	missing := comments.CIDOf([]byte("never stored anywhere"))
	subject := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/v1/peers/%s/comments/%s", subject.String(), hex.EncodeToString(missing)), nil)
	b.handlePeersForTest(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status=%d; want 404", rec.Code)
	}
}

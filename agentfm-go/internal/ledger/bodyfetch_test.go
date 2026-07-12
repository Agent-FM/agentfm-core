package ledger

import (
	"context"
	"path/filepath"
	"testing"

	"agentfm/internal/ledger/comments"
	"agentfm/test/testutil"
)

func TestBodyBackfill_FetchesMissingBodies(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 3)
	authorHost, archiveHost, subjectHost := hosts[0], hosts[1], hosts[2]

	authorStore, err := comments.Open(filepath.Join(t.TempDir(), "author"))
	if err != nil {
		t.Fatalf("open author store: %v", err)
	}
	srv := comments.NewServer(authorHost, authorStore)
	srv.Start()
	t.Cleanup(srv.Stop)

	body := []byte("backfilled after the live fetch was missed")
	cid, err := authorStore.Put(body)
	if err != nil {
		t.Fatalf("author Put: %v", err)
	}

	s := testutil.OpenTestStore(t)
	testutil.AppendInboxComment(t, s, authorHost, subjectHost.ID(), cid)

	archiveStore, err := comments.Open(filepath.Join(t.TempDir(), "archive"))
	if err != nil {
		t.Fatalf("open archive store: %v", err)
	}
	f := newBodyFetcher(archiveHost, archiveStore)
	f.backfillOnce(context.Background(), s)

	if !archiveStore.Has(cid) {
		t.Fatal("backfillOnce should have fetched and stored the missing body")
	}

	got, err := archiveStore.Get(cid)
	if err != nil {
		t.Fatalf("Get after backfill: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("backfilled body mismatch: %q", got)
	}
}

func TestBodyBackfill_SkipsBodiesAlreadyStored(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	authorHost, archiveHost := hosts[0], hosts[1]

	s := testutil.OpenTestStore(t)
	archiveStore, err := comments.Open(filepath.Join(t.TempDir(), "archive"))
	if err != nil {
		t.Fatalf("open archive store: %v", err)
	}

	body := []byte("already present locally")
	cid, err := archiveStore.Put(body)
	if err != nil {
		t.Fatalf("archive Put: %v", err)
	}
	testutil.AppendInboxComment(t, s, authorHost, archiveHost.ID(), cid)

	f := newBodyFetcher(archiveHost, archiveStore)
	f.backfillOnce(context.Background(), s)

	if !archiveStore.Has(cid) {
		t.Fatal("existing body must remain in the store")
	}
}

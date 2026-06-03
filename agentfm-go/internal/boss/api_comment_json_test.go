package boss

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
)

func TestCommentBodyJSON_ReturnsWrappedBody(t *testing.T) {
	// Build a minimal boss with a commentsStore — no ledger required for
	// the GET path, but we include one for completeness.
	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		commentsStore: cs,
	}

	// Pre-store a comment body.
	cid, err := cs.Put([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	cidHex := hex.EncodeToString(cid)

	subj := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/peers/%s/comments/%s.json", subj.String(), cidHex), nil)
	b.handlePeersForTest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if got["body"] != "hello world" {
		t.Fatalf("body mismatch: %v", got["body"])
	}
	if got["cid"] != cidHex {
		t.Fatalf("cid mismatch: got %v, want %v", got["cid"], cidHex)
	}
}

func TestCommentBodyJSON_PlainTextStillWorks(t *testing.T) {
	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		commentsStore: cs,
	}

	// Pre-store a comment body.
	cid, err := cs.Put([]byte("plain text response"))
	if err != nil {
		t.Fatal(err)
	}
	cidHex := hex.EncodeToString(cid)

	subj := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/peers/%s/comments/%s", subj.String(), cidHex), nil)
	b.handlePeersForTest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "plain text response" {
		t.Fatalf("body mismatch: %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q; want text/plain; charset=utf-8", ct)
	}
}

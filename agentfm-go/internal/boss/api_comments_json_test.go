package boss

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
)

func TestHandleCommentBodyGetJSON_ContentTypeAndBody(t *testing.T) {
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

	const stored = "content-type regression body"
	cid, err := cs.Put([]byte(stored))
	if err != nil {
		t.Fatalf("store.Put: %v", err)
	}
	cidHex := hex.EncodeToString(cid)

	subj := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/v1/peers/%s/comments/%s.json", subj.String(), cidHex), nil)
	b.handleCommentBodyGetJSON(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q; want it to contain application/json", ct)
	}

	var got commentBodyJSONResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if got.Body != stored {
		t.Fatalf("body = %q; want %q", got.Body, stored)
	}
	if got.CID != cidHex {
		t.Fatalf("cid = %q; want %q", got.CID, cidHex)
	}
}

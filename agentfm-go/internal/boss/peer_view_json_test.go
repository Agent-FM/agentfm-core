package boss

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agentfm/internal/ledger/comments"
)

// TestPeerEntry_TextCIDMarshalsAsHex pins the wire contract: text_cid in the
// /v1/peers/{id}/log response MUST be lowercase hex, matching the format
// accepted by handleCommentBodyGet's URL parser (hex.DecodeString).
//
// Regression test for the desktop "comments do not load" bug: the JSON output
// was emitting base64-std (Go's default for []byte) while the URL handler
// expected hex, causing every comment-body fetch to 400 with
// "CID is not valid hex".
func TestPeerEntry_TextCIDMarshalsAsHex(t *testing.T) {
	// Use a real multihash-shaped CID so the test exercises the full 34-byte path.
	body := []byte("hello world")
	cid := comments.CIDOf(body)

	entry := PeerEntry{
		Kind:    "Comment",
		TextCID: cid,
	}

	blob, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	wantHex := hex.EncodeToString(cid)
	wantField := `"text_cid":"` + wantHex + `"`
	if !strings.Contains(string(blob), wantField) {
		t.Fatalf("expected %s in JSON output; got: %s", wantField, string(blob))
	}
}

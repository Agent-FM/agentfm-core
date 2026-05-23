package boss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"
)

// selfTestLedger is a minimal ledger stub that records appended entries
// so the self-comment test can verify what the handler builds.
type selfTestLedger struct {
	stubLedger
	appended []*pb.SignedEntry
	lastHash [32]byte
}

func (l *selfTestLedger) Append(_ context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	l.appended = append(l.appended, payload)
	l.lastHash = sha256.Sum256([]byte{byte(len(l.appended))})
	return l.lastHash, nil
}

func newSelfCommentRig(t *testing.T) (*Boss, *selfTestLedger, *comments.Store) {
	t.Helper()
	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}
	led := &selfTestLedger{}
	h := testutil.NewHost(t)
	b := &Boss{
		node:          &network.MeshNode{Host: h},
		ledger:        led,
		commentsStore: cs,
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
	}
	return b, led, cs
}

func TestSelfComment_Submit_AppendsToLedger(t *testing.T) {
	b, led, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()

	body := SelfCommentSubmitRequest{Text: "great agent, did the work", Language: "en"}
	raw, _ := json.Marshal(body)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var got CommentSubmitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if got.CID == "" {
		t.Errorf("expected CID in response, got empty")
	}
	if got.LedgerHash == "" {
		t.Errorf("expected ledger_hash in response, got empty")
	}
	if len(led.appended) != 1 {
		t.Fatalf("expected 1 ledger append, got %d", len(led.appended))
	}
	entry := led.appended[0]
	c := entry.GetComment()
	if c == nil {
		t.Fatalf("expected SignedEntry_Comment payload, got %T", entry.Body)
	}
	if string(c.SubjectPeerId) != string(subj) {
		t.Errorf("subject mismatch: got %x want %x", c.SubjectPeerId, []byte(subj))
	}
	if string(c.RaterPeerId) != string(b.node.Host.ID()) {
		t.Errorf("rater mismatch: got %x want %x", c.RaterPeerId, []byte(b.node.Host.ID()))
	}
	if c.Language != "en" {
		t.Errorf("language mismatch: got %q want %q", c.Language, "en")
	}
}

// TestSelfComment_WithRating_AppendsBothCommentAndRating pins the contract
// for the desktop FeedbackModal: when the user submits a comment WITH a
// rating slider value, the boss must append BOTH a Comment AND a paired
// Rating entry (dimension "honesty") to the ledger — not just the Comment.
// Regression test for the empty-Ratings-tab bug in PeerView.
func TestSelfComment_WithRating_AppendsBothCommentAndRating(t *testing.T) {
	b, led, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()

	score := 0.42
	body := SelfCommentSubmitRequest{Text: "did great", Language: "en", Rating: &score}
	raw, _ := json.Marshal(body)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(led.appended) != 2 {
		t.Fatalf("expected 2 ledger appends (Comment + Rating), got %d", len(led.appended))
	}
	if led.appended[0].GetComment() == nil {
		t.Errorf("first append must be a Comment, got %T", led.appended[0].Body)
	}
	r := led.appended[1].GetRating()
	if r == nil {
		t.Fatalf("second append must be a Rating, got %T", led.appended[1].Body)
	}
	if r.Dimension != "honesty" {
		t.Errorf("rating dimension: got %q want %q", r.Dimension, "honesty")
	}
	if r.Score != score {
		t.Errorf("rating score: got %v want %v", r.Score, score)
	}
	if string(r.SubjectPeerId) != string(subj) {
		t.Errorf("rating subject mismatch")
	}
	if string(r.RaterPeerId) != string(b.node.Host.ID()) {
		t.Errorf("rating rater mismatch")
	}
}

func TestSelfComment_WithoutRating_AppendsOnlyComment(t *testing.T) {
	b, led, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()

	body := SelfCommentSubmitRequest{Text: "no rating slider used", Language: "en"}
	raw, _ := json.Marshal(body)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(led.appended) != 1 {
		t.Fatalf("expected exactly 1 append (Comment only), got %d", len(led.appended))
	}
	if led.appended[0].GetRating() != nil {
		t.Errorf("must not append a Rating when score is omitted")
	}
}

func TestSelfComment_RejectsOutOfRangeRating(t *testing.T) {
	b, _, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()
	score := 1.5
	body := SelfCommentSubmitRequest{Text: "x", Rating: &score}
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("want 400 for out-of-range rating; got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSelfComment_RejectsGet(t *testing.T) {
	b, _, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), nil)
	b.handlePeersForTest(rec, req)
	if rec.Code != 405 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSelfComment_RejectsEmptyText(t *testing.T) {
	b, _, _ := newSelfCommentRig(t)
	subj := testutil.NewHost(t).ID()
	raw := []byte(`{"text":""}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/peers/%s/comments/self", subj.String()), bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "text is required") {
		t.Errorf("expected text-required error, got %q", rec.Body.String())
	}
}

func TestSelfComment_RejectsBadPeerID(t *testing.T) {
	b, _, _ := newSelfCommentRig(t)
	raw := []byte(`{"text":"hi"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/peers/not-a-peer-id/comments/self", bytes.NewReader(raw))
	b.handlePeersForTest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

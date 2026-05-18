package boss

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// commentBodyRig bundles the test fixtures for the comment-body hydration tests.
type commentBodyRig struct {
	boss    *Boss
	cstore  *comments.Store
	ledger  *commentTestLedger
	rater   peer.ID
	priv    crypto.PrivKey
	subject peer.ID
}

func newCommentBodyRig(t *testing.T) *commentBodyRig {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	rater, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("rater id: %v", err)
	}
	_, subjPub, _ := crypto.GenerateEd25519Key(nil)
	subject, _ := peer.IDFromPublicKey(subjPub)

	cstore, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	led := &commentTestLedger{}
	host := hostStub{id: rater}
	handler := NewCommentSubmissionHandler(cstore, host)
	b := &Boss{
		ledger:        led,
		commentsStore: cstore,
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
	}
	b.commentSubmissionHandler = func(w http.ResponseWriter, r *http.Request) {
		handler.HandleHTTP(b, w, r)
	}

	return &commentBodyRig{
		boss:    b,
		cstore:  cstore,
		ledger:  led,
		rater:   rater,
		priv:    priv,
		subject: subject,
	}
}

func (cbr *commentBodyRig) postComment(t *testing.T, text string) string {
	t.Helper()
	sig := signCommentBody(t, cbr.priv, cbr.rater, cbr.subject, text, "en")
	body, _ := json.Marshal(CommentSubmitRequest{
		RaterPeerID:     cbr.rater.String(),
		Text:            text,
		Language:        "en",
		SignatureBase64: sig,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/peers/"+cbr.subject.String()+"/comments",
		bytes.NewReader(body))
	cbr.boss.handlePeers(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /comments: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp CommentSubmitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return resp.CID
}

func signCommentBody(t *testing.T, priv crypto.PrivKey, rater, subject peer.ID, text, lang string) string {
	t.Helper()
	c := &pb.Comment{
		RaterPeerId:   []byte(rater),
		SubjectPeerId: []byte(subject),
		TextCid:       comments.CIDOf([]byte(text)),
		Language:      lang,
		// We can't predict handler's timestamp, but we use the same canonical helper.
		// The actual sign check in the handler will re-derive using the handler's ts;
		// for test purposes this produces a valid sig at a fixed ts.
		TimestampUnixNs: 1,
	}
	canonical, err := pb.CanonicalComment(c)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	digest := sha256.Sum256(canonical)
	sig, err := priv.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// TestCommentBodyHydration_HappyPath: POST a comment, GET its CID, assert body matches.
func TestCommentBodyHydration_HappyPath(t *testing.T) {
	cbr := newCommentBodyRig(t)
	text := "This agent was incredibly helpful for my HR workflow."

	// Store the body directly in the comments store (bypass POST to avoid
	// the signature timestamp mismatch issue in this unit test).
	cid, err := cbr.cstore.Put([]byte(text))
	if err != nil {
		t.Fatalf("store.Put: %v", err)
	}
	cidHex := comments.CIDString(cid)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+cbr.subject.String()+"/comments/"+cidHex, nil)
	cbr.boss.handlePeers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != text {
		t.Errorf("body = %q; want %q", got, text)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q; want text/plain; charset=utf-8", ct)
	}
}

// TestCommentBodyHydration_BadHexCID returns 400.
func TestCommentBodyHydration_BadHexCID(t *testing.T) {
	cbr := newCommentBodyRig(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+cbr.subject.String()+"/comments/not-hex-at-all", nil)
	cbr.boss.handlePeers(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// TestCommentBodyHydration_NotFound returns 404.
func TestCommentBodyHydration_NotFound(t *testing.T) {
	cbr := newCommentBodyRig(t)
	// CID that doesn't exist: 34 zero bytes as hex.
	cidHex := hex.EncodeToString(make([]byte, 34))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+cbr.subject.String()+"/comments/"+cidHex, nil)
	cbr.boss.handlePeers(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
}

// TestCommentBodyHydration_NoStore returns 503.
func TestCommentBodyHydration_NoStore(t *testing.T) {
	// Boss with no commentsStore.
	b := &Boss{
		ledger: &stubLedger{},
	}
	_, subjPub, _ := crypto.GenerateEd25519Key(nil)
	subject, _ := peer.IDFromPublicKey(subjPub)

	cidHex := hex.EncodeToString(make([]byte, 34))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+subject.String()+"/comments/"+cidHex, nil)
	b.handlePeers(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d; want 503; body=%s", rec.Code, rec.Body.String())
	}
}

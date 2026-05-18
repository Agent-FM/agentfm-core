package boss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
)

// stubLedger is a minimal Ledger implementation that satisfies the
// interface so http tests can exercise handler paths without
// standing up a real SQLite + libp2p stack.
type stubLedger struct{}

func (stubLedger) Append(ctx context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	return [32]byte{}, nil
}
func (stubLedger) Head(ctx context.Context) (*pb.LogHead, error) { return nil, nil }
func (stubLedger) Prove(ctx context.Context, entryHash [32]byte) (*pb.InclusionProof, error) {
	return nil, ledger.ErrEntryNotInLog
}
func (stubLedger) VerifyEntry(ctx context.Context, entry *pb.SignedEntry, knownHead *pb.LogHead) error {
	return nil
}
func (stubLedger) InboxHas(ctx context.Context, raterID []byte, entryHash [32]byte) (bool, error) {
	return false, nil
}
func (stubLedger) IsEquivocator(ctx context.Context, peerID []byte) (bool, error) { return false, nil }
func (stubLedger) AcceptEntry(ctx context.Context, payload []byte) error          { return nil }
func (stubLedger) LastInboxIdx(ctx context.Context) (uint64, error)               { return 0, nil }
func (stubLedger) Store() *store.Store                                            { return nil }
func (stubLedger) Close() error                                                   { return nil }

// compile-time check
var _ ledger.Ledger = (*stubLedger)(nil)

// httpTestBoss returns a Boss that has no libp2p node and a nil
// ledger — sufficient for tests that exercise the handler's
// error-path and routing logic without needing a real ledger.
func httpTestBoss() *Boss {
	return &Boss{}
}

func TestHandlePeers_RoutesByPath(t *testing.T) {
	b := httpTestBoss()
	cases := []struct {
		path     string
		wantBody string // partial match
	}{
		{"/v1/peers/12D3KooW/reputation", "ledger_unavailable"},
		{"/v1/peers/12D3KooW/log", "ledger_unavailable"},
		{"/v1/peers/12D3KooW/proof", "ledger_unavailable"},
		{"/v1/peers/12D3KooW/comments", "not_implemented"},
		{"/v1/peers/12D3KooW/bogus", "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			b.handlePeers(rec, req)
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body = %q, want substring %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestHandleReputation_MissingPeerID(t *testing.T) {
	// Boss has a nil ledger so we'd normally short-circuit with
	// 503; spin one with a stub so we exercise the path-parse path.
	b := &Boss{ledger: &stubLedger{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/peers//reputation", nil)
	b.handleReputation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleReputation_BadPeerID(t *testing.T) {
	b := &Boss{ledger: &stubLedger{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/peers/not-a-peer-id/reputation", nil)
	b.handleReputation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleProof_BadEntryParam(t *testing.T) {
	b := &Boss{ledger: &stubLedger{}}
	cases := []string{
		"",
		"not-hex",
		"abc", // too short
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			rec := httptest.NewRecorder()
			url := "/v1/peers/12D3KooW/proof"
			if c != "" {
				url += "?entry=" + c
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			b.handleProof(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("entry=%q: status = %d, want 400", c, rec.Code)
			}
		})
	}
}

func TestExtractPeerID(t *testing.T) {
	cases := []struct {
		path, suffix, want string
	}{
		{"/v1/peers/abc/reputation", "/reputation", "abc"},
		{"/v1/peers/abc/log", "/log", "abc"},
		{"/v1/peers/abc/proof", "/proof", "abc"},
		{"/v1/peers/abc", "", "abc"},
		{"/v1/peers/abc/comments", "/comments", "abc"},
		{"/v1/something/else", "", ""},
		{"/v1/peers/", "", ""},
	}
	for _, tc := range cases {
		if got := extractPeerID(tc.path, tc.suffix); got != tc.want {
			t.Errorf("extractPeerID(%q, %q) = %q, want %q", tc.path, tc.suffix, got, tc.want)
		}
	}
}

// JSON envelope shape: ensure reputation responses serialise cleanly.
func TestReputationResponse_JSONShape(t *testing.T) {
	r := reputationResponse{
		PeerID:        "12D3KooW",
		Scores:        map[string]float64{"honesty": -1.0},
		IsEquivocator: true,
	}
	bs, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(bs)
	if !strings.Contains(s, `"is_equivocator":true`) {
		t.Errorf("missing is_equivocator field: %s", s)
	}
	if !strings.Contains(s, `"honesty":-1`) {
		t.Errorf("missing honesty score: %s", s)
	}
}

package boss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger/store"
	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func newPeerIDPS(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	id, _ := peer.IDFromPublicKey(pub)
	return id
}

func openFreshPS(t *testing.T) *store.Store {
	t.Helper()
	return openFreshPV(t) // reuse same helper from peer_view_test.go
}

// TestHandlePeerGet_AllFields asserts GET /v1/peers/{id} returns a
// well-formed summary with all required fields.
func TestHandlePeerGet_AllFields(t *testing.T) {
	s := openFreshPS(t)
	rater := newPeerIDPS(t)
	subject := newPeerIDPS(t)

	// Insert a rating so entries_count > 0.
	insertInboxRating(t, s, rater, subject, 0.6)

	b := &Boss{
		ledger:        &stubLedger{},
		readStore:     s,
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/peers/"+subject.String(), nil)
	b.handlePeers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mustHaveKeys := []string{
		"peer_id",
		"agent_name",
		"online",
		"honesty_score",
		"is_equivocator",
		"dispatch_allowed",
		// dispatch_refuse_reason is omitempty — absent when empty string
		"entries_count",
		"rater_summary",
	}
	for _, k := range mustHaveKeys {
		if _, ok := resp[k]; !ok {
			t.Errorf("key %q missing from response; keys present: %v", k, mapKeysAny(resp))
		}
	}

	if pid, ok := resp["peer_id"].(string); !ok || pid != subject.String() {
		t.Errorf("peer_id = %v; want %s", resp["peer_id"], subject.String())
	}

	raterSummary, ok := resp["rater_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("rater_summary is not a map; got %T", resp["rater_summary"])
	}
	for _, k := range []string{"verified_raters_count", "unverified_raters_count"} {
		if _, ok := raterSummary[k]; !ok {
			t.Errorf("rater_summary.%q missing", k)
		}
	}

	// entries_count must be >= 1.
	if ec, ok := resp["entries_count"].(float64); !ok || ec < 1 {
		t.Errorf("entries_count = %v; want >= 1", resp["entries_count"])
	}

	// online defaults false for peers not in activeWorkers.
	if online, ok := resp["online"].(bool); !ok || online {
		t.Errorf("online should be false for peer not in activeWorkers; got %v", resp["online"])
	}
}

// TestHandlePeerGet_OnlineFromActiveWorkers asserts online=true when the
// peer has an active telemetry profile.
func TestHandlePeerGet_OnlineFromActiveWorkers(t *testing.T) {
	s := openFreshPS(t)
	subject := newPeerIDPS(t)

	b := &Boss{
		ledger:    &stubLedger{},
		readStore: s,
		activeWorkers: map[string]types.WorkerProfile{
			subject.String(): {
				PeerID:           subject.String(),
				AgentName:        "my-agent",
				AgentImageRef:    "ghcr.io/test:v1",
				AgentImageDigest: "sha256:deadbeef",
				AgentCapability:  "ml",
			},
		},
		lastSeen: map[string]time.Time{subject.String(): time.Now()},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/peers/"+subject.String(), nil)
	b.handlePeers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if online, ok := resp["online"].(bool); !ok || !online {
		t.Errorf("online should be true for peer in activeWorkers; got %v", resp["online"])
	}
	if name, ok := resp["agent_name"].(string); !ok || name != "my-agent" {
		t.Errorf("agent_name = %v; want my-agent", resp["agent_name"])
	}
	if ref, ok := resp["advertised_image_ref"].(string); !ok || ref != "ghcr.io/test:v1" {
		t.Errorf("advertised_image_ref = %v; want ghcr.io/test:v1", resp["advertised_image_ref"])
	}

	// Ensure context import usage
	_ = context.Background()
}

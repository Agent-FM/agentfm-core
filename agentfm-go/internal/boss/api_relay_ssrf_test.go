package boss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/test/testutil"
)

// M1: /api/relay/test must refuse to dial private / link-local ranges
// (SSRF port-scan oracle when the API is exposed off-host), while still
// allowing loopback (the desktop's own local relay).
func TestHandleRelayTest_RejectsPrivateIP(t *testing.T) {
	pid := testutil.NewHost(t).ID().String()
	b := &Boss{}

	for _, addr := range []string{
		"/ip4/10.0.0.5/tcp/4001/p2p/" + pid,
		"/ip4/192.168.1.10/tcp/4001/p2p/" + pid,
		"/ip4/172.16.0.9/tcp/4001/p2p/" + pid,
		"/ip4/169.254.169.254/tcp/80/p2p/" + pid,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/relay/test",
			strings.NewReader(`{"multiaddr":"`+addr+`"}`))
		b.handleRelayTest(rec, req)

		var resp relayTestResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode: %v", addr, err)
		}
		if resp.OK {
			t.Fatalf("%s: expected ok=false", addr)
		}
		if !strings.Contains(resp.Error, "disallowed") && !strings.Contains(resp.Error, "private") {
			t.Fatalf("%s: expected disallowed-range error, got: %q", addr, resp.Error)
		}
	}
}

func TestHandleRelayTest_AllowsLoopback(t *testing.T) {
	pid := testutil.NewHost(t).ID().String()
	b := &Boss{} // no host: a loopback addr must pass the filter and hit the host-check

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/relay/test",
		strings.NewReader(`{"multiaddr":"/ip4/127.0.0.1/tcp/4015/p2p/`+pid+`"}`))
	b.handleRelayTest(rec, req)

	var resp relayTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(resp.Error, "disallowed") {
		t.Fatalf("loopback wrongly filtered: %q", resp.Error)
	}
	if !strings.Contains(resp.Error, "host not initialized") {
		t.Fatalf("expected host-not-initialized (proving loopback passed the filter), got: %q", resp.Error)
	}
}

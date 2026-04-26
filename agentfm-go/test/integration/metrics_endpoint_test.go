package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/internal/metrics"
)

// TestMetricsEndpoint_ListsAllAgentFMFamilies asserts that the /metrics
// handler the boss-api mounts exposes every agentfm_* family this release
// promises. We do not boot the full boss here — that path is exercised by
// the operator smoke test in README — instead we mount the same handler
// behind httptest, since the wiring is one line and the contract under test
// is purely the metrics package's output shape.
func TestMetricsEndpoint_ListsAllAgentFMFamilies(t *testing.T) {
	srv := httptest.NewServer(metrics.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	bodyStr := string(body)

	for _, family := range []string{
		"agentfm_tasks_total",
		"agentfm_task_duration_seconds",
		"agentfm_workers_online",
		"agentfm_artifact_bytes_sent_total",
		"agentfm_stream_errors_total",
		"agentfm_dht_queries_total",
	} {
		if !strings.Contains(bodyStr, family) {
			t.Errorf("expected family %q in /metrics scrape, not found", family)
		}
	}

	// Sanity: prometheus content-type is set so scrapers parse cleanly.
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") && !strings.Contains(ct, "openmetrics") {
		t.Errorf("Content-Type=%q, want text/plain or openmetrics variant", ct)
	}
}

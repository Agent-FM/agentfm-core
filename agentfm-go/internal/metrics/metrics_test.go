package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// scrapeBody hits the package Handler() over httptest and returns the body.
// Keeping this helper thin so the assertions below stay focused on what
// each test cares about.
func scrapeBody(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestRegistryContainsAllAgentFMFamilies(t *testing.T) {
	t.Parallel()
	families, err := Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	got := map[string]bool{}
	for _, f := range families {
		got[f.GetName()] = true
	}
	wantNames := []string{
		"agentfm_tasks_total",
		"agentfm_task_duration_seconds",
		"agentfm_workers_online",
		"agentfm_artifact_bytes_sent_total",
		"agentfm_stream_errors_total",
		"agentfm_dht_queries_total",
	}
	for _, name := range wantNames {
		if !got[name] {
			t.Errorf("expected metric %q to be registered, not found", name)
		}
	}
}

func TestHandlerExposesGoRuntimeCollectors(t *testing.T) {
	t.Parallel()
	body := scrapeBody(t)
	for _, name := range []string{"go_goroutines", "process_resident_memory_bytes"} {
		if !strings.Contains(body, name) {
			t.Errorf("expected runtime metric %q in /metrics output, not found", name)
		}
	}
}

func TestTasksTotalIncrements(t *testing.T) {
	// Not parallel — mutates the package-global counter.
	before := readCounterVec(t, "agentfm_tasks_total", "ok")
	TasksTotal.WithLabelValues(StatusOK).Inc()
	after := readCounterVec(t, "agentfm_tasks_total", "ok")
	if after-before != 1 {
		t.Fatalf("counter delta=%v, want 1", after-before)
	}
}

func TestTaskDurationHistogramHonorsAgentBuckets(t *testing.T) {
	body := scrapeBody(t)
	// Spot-check a couple of bucket boundaries we declared explicitly
	// (1s and 180s); ensures the histogram isn't using default buckets.
	for _, marker := range []string{`le="1"`, `le="180"`, `le="1800"`} {
		if !strings.Contains(body, marker) {
			t.Errorf("expected histogram bucket %q in /metrics output, not found", marker)
		}
	}
}

func TestStreamErrorsTotalLabelsExposed(t *testing.T) {
	StreamErrorsTotal.WithLabelValues(ProtocolTask, ReasonReset).Inc()
	body := scrapeBody(t)
	if !strings.Contains(body, `agentfm_stream_errors_total{protocol="task",reason="reset"}`) {
		t.Fatalf("expected labelled stream-error sample in body; got:\n%s", body)
	}
}

// readCounterVec parses a single CounterVec sample value out of /metrics
// for the given label sequence (positional, must match the metric's label
// names in declaration order).
func readCounterVec(t *testing.T, name, labelValue string) float64 {
	t.Helper()
	body := scrapeBody(t)
	// Look for: name{label="value"} <float>
	prefix := name + `{`
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if !strings.Contains(line, `"`+labelValue+`"`) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		var v float64
		if _, err := fmt.Sscan(parts[len(parts)-1], &v); err != nil {
			t.Fatalf("parse %q: %v", parts[len(parts)-1], err)
		}
		return v
	}
	return 0
}

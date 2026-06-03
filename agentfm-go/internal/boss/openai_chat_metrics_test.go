package boss

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/types"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestChatCompletion_IncrementsTasksTotalOnSuccess(t *testing.T) {
	const reply = "hello from worker"
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, reply),
	)
	before := tasksCount(t, "ok")

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := tasksCount(t, "ok") - before; got != 1 {
		t.Fatalf("TasksTotal{ok}: grew by %v, want 1", got)
	}
}

func TestChatCompletion_IncrementsTasksTotalOnFailure(t *testing.T) {
	failHandler := func(s netcore.Stream) {
		_ = s.Reset()
	}
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		failHandler,
	)
	before := tasksCount(t, "error")

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if got := tasksCount(t, "error") - before; got != 1 {
		t.Fatalf("TasksTotal{error}: grew by %v, want 1 (status=%d body=%s)",
			got, rec.Code, rec.Body.String())
	}
}

func TestChatCompletion_StreamingIncrementsTasksTotalOnSuccess(t *testing.T) {
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, "alpha\n", "bravo\n"),
	)
	before := tasksCount(t, "ok")

	srv := httptest.NewServer(http.HandlerFunc(b.handleChatCompletions))
	t.Cleanup(srv.Close)

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req, err := http.NewRequestWithContext(testCtx(t, 10*time.Second), http.MethodPost, srv.URL, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		_ = sc.Text()
	}
	_, _ = io.Copy(io.Discard, resp.Body)

	if got := tasksCount(t, "ok") - before; got != 1 {
		t.Fatalf("TasksTotal{ok} (stream): grew by %v, want 1", got)
	}
}

func tasksCount(t testing.TB, status string) float64 {
	t.Helper()
	c, err := metrics.TasksTotal.GetMetricWithLabelValues(status)
	if err != nil {
		t.Fatalf("TasksTotal lookup %q: %v", status, err)
	}
	m := &dto.Metric{}
	if err := c.(prometheus.Counter).Write(m); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	return m.GetCounter().GetValue()
}

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/boss"
	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	dto "github.com/prometheus/client_model/go"
)

// TestTasksTotal_IncrementsOnSuccess wires two real libp2p hosts: one is a
// minimal worker stub that accepts the TaskProtocol stream and echoes a
// fixed response; the other hosts a Boss whose /api/execute handler we
// invoke directly. After the handler returns we assert the OK counter
// went up by exactly one.
func TestTasksTotal_IncrementsOnSuccess(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	workerHost, bossHost := hosts[0], hosts[1]

	registerEchoWorker(t, workerHost)

	b := boss.NewForTest(&network.MeshNode{Host: bossHost})
	b.SeedWorker(types.WorkerProfile{
		PeerID:    workerHost.ID().String(),
		AgentName: "echo",
		CPUCores:  1,
		MaxTasks:  1,
		Status:    "AVAILABLE",
	})

	before := readCounter(t, "agentfm_tasks_total", map[string]string{"status": metrics.StatusOK})

	body := mustJSON(map[string]string{
		"worker_id": workerHost.ID().String(),
		"prompt":    "hi",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute", bytes.NewReader(body))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}

	testutil.Eventually(t, 2*time.Second, func() bool {
		return readCounter(t, "agentfm_tasks_total",
			map[string]string{"status": metrics.StatusOK}) >= before+1
	}, "expected agentfm_tasks_total{status=ok} to increment by 1")
}

// TestStreamErrorsTotal_IncrementsOnDecodeFailure: send malformed JSON to a
// stub TaskProtocol handler that mirrors the worker's decode logic and
// assert the decode counter went up.
func TestStreamErrorsTotal_IncrementsOnDecodeFailure(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	a, b := hosts[0], hosts[1]

	a.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		_ = s.SetDeadline(time.Now().Add(network.TaskPayloadReadTimeout))
		var p types.TaskPayload
		if err := json.NewDecoder(io.LimitReader(s, 1024)).Decode(&p); err != nil {
			metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolTask, metrics.ReasonDecode).Inc()
			_ = s.Reset()
			return
		}
		_ = s.Close()
	})

	before := readCounter(t, "agentfm_stream_errors_total",
		map[string]string{"protocol": metrics.ProtocolTask, "reason": metrics.ReasonDecode})

	stream, err := b.NewStream(testutil.WithTimeout(t, 5*time.Second), a.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if _, err := stream.Write([]byte("not json at all")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_ = stream.CloseWrite()

	testutil.Eventually(t, 2*time.Second, func() bool {
		return readCounter(t, "agentfm_stream_errors_total",
			map[string]string{"protocol": metrics.ProtocolTask, "reason": metrics.ReasonDecode}) >= before+1
	}, "expected agentfm_stream_errors_total{protocol=task,reason=decode} to increment")
}

// registerEchoWorker mounts a stub TaskProtocol handler that drains the
// JSON payload, writes a fixed response, and closes. Minimum viable
// worker for testing call-counter behaviour without dragging Podman in.
func registerEchoWorker(t *testing.T, h host.Host) {
	t.Helper()
	h.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		_ = s.SetDeadline(time.Now().Add(10 * time.Second))
		var p types.TaskPayload
		_ = json.NewDecoder(io.LimitReader(s, 1024*1024)).Decode(&p)
		_, _ = s.Write([]byte("hello\n"))
		_, _ = s.Write([]byte("\n[AGENTFM: NO_FILES]\n"))
		_ = s.Close()
	})
}

func readCounter(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			if !labelsMatch(m.GetLabel(), labels) {
				continue
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func labelsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	have := map[string]string{}
	for _, p := range got {
		have[p.GetName()] = p.GetValue()
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func mustJSON(v any) []byte {
	bts, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("test helper marshal: %v", err))
	}
	return bts
}

package worker

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/internal/version"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// --- Task stream rejection paths -------------------------------------------

// TestHandleTaskStream_RejectsWhenAtCapacity verifies the first circuit
// breaker: a worker already at maxtasks must reject immediately with a
// human-readable message and close gracefully (not Reset) so the Boss sees
// the reason.
func TestHandleTaskStream_RejectsWhenAtCapacity(t *testing.T) {
	w, workerHost := newTestWorker(t, Config{MaxConcurrentTasks: 2, MaxCPU: 90, MaxGPU: 90})
	w.currentTasks = 2 // synthetic saturation

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	// Send a well-formed payload that would otherwise succeed. The worker
	// should reject before it even tries to decode us.
	payload, _ := json.Marshal(types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    "hello",
	})
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = s.CloseWrite()

	resp, err := io.ReadAll(s)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	_ = s.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not finish")
	}

	if !strings.Contains(string(resp), "max capacity") {
		t.Errorf("want capacity rejection message, got: %q", resp)
	}
}

// TestHandleTaskStream_RejectsWhenCPUOverloaded exercises the CPU circuit
// breaker. currentCPU is already above maxcpu, so the worker must respond
// with an overload message before trying to execute the task.
func TestHandleTaskStream_RejectsWhenCPUOverloaded(t *testing.T) {
	w, workerHost := newTestWorker(t, Config{MaxConcurrentTasks: 10, MaxCPU: 50, MaxGPU: 90})
	w.currentCPU = 95.0 // synthetic overload

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	payload, _ := json.Marshal(types.TaskPayload{
		Version: version.AppVersion, Task: "agent_task", Data: "x",
	})
	_, _ = s.Write(payload)
	_ = s.CloseWrite()

	resp, _ := io.ReadAll(s)
	_ = s.Close()
	<-done

	if !strings.Contains(string(resp), "heavy load") && !strings.Contains(string(resp), "CPU") {
		t.Errorf("want CPU overload message, got: %q", resp)
	}
}

// TestHandleTaskStream_VersionMismatch: payload with a non-matching version
// string triggers a human-readable rejection — also close-not-reset, so the
// Boss sees the specific version the worker expects.
func TestHandleTaskStream_VersionMismatch(t *testing.T) {
	w, workerHost := newTestWorker(t, Config{MaxConcurrentTasks: 10, MaxCPU: 99, MaxGPU: 99})

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	payload, _ := json.Marshal(types.TaskPayload{
		Version: "99.99.99-wrong", Task: "agent_task", Data: "x",
	})
	_, _ = s.Write(payload)
	_ = s.CloseWrite()

	resp, _ := io.ReadAll(s)
	_ = s.Close()
	<-done

	if !strings.Contains(string(resp), "Version mismatch") {
		t.Errorf("want version mismatch message, got: %q", resp)
	}
}

// TestHandleTaskStream_InvalidJSON: malformed payload must trigger the
// Reset path (not graceful close). We can't directly observe Reset vs
// Close at the peer level, but we can assert no rejection message was
// written AND the stream returns an error or empty to the peer.
func TestHandleTaskStream_InvalidJSON(t *testing.T) {
	w, workerHost := newTestWorker(t, Config{MaxConcurrentTasks: 10, MaxCPU: 99, MaxGPU: 99})

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_, _ = s.Write([]byte("this is not json at all {{{"))
	_ = s.CloseWrite()

	// Reset streams should cause ReadAll to return an error OR an empty
	// buffer depending on timing. We only assert no rejection-style message
	// leaked back, since the handler should NOT write anything on decode
	// failure.
	resp, _ := io.ReadAll(s)
	_ = s.Close()
	<-done

	if strings.Contains(string(resp), "ERROR") || strings.Contains(string(resp), "mismatch") {
		t.Errorf("expected no rejection message on decode fail, got: %q", resp)
	}
}

// TestHandleTaskStream_PayloadTooLarge: the 1MB io.LimitReader cap is the
// OOM shield. Anything beyond gets truncated; json.Decode fails mid-parse;
// handler Resets. Regression guard for §1.1 "Always wrap stream readers
// with io.LimitReader before decoding JSON".
func TestHandleTaskStream_PayloadTooLarge(t *testing.T) {
	w, workerHost := newTestWorker(t, Config{MaxConcurrentTasks: 10, MaxCPU: 99, MaxGPU: 99})

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	// 2MB prompt — well over the 1MB LimitReader. Decode must fail.
	bigData := strings.Repeat("a", 2*1024*1024)
	payload, _ := json.Marshal(types.TaskPayload{
		Version: version.AppVersion, Task: "agent_task", Data: bigData,
	})
	_, _ = s.Write(payload)
	_ = s.CloseWrite()

	resp, _ := io.ReadAll(s)
	_ = s.Close()
	<-done

	// Same assertion as InvalidJSON — decode failure is silent at the peer.
	if strings.Contains(string(resp), "ERROR") {
		t.Errorf("expected no rejection message on oversized payload, got: %q", resp)
	}
}

// --- Feedback stream -------------------------------------------------------

// TestHandleFeedbackStream_WritesLog verifies the happy path: valid JSON
// feedback arrives and gets persisted to feedback.log in the agent dir.
func TestHandleFeedbackStream_WritesLog(t *testing.T) {
	agentDir := t.TempDir()
	w, workerHost := newTestWorker(t, Config{
		MaxConcurrentTasks: 1,
		AgentDir:           agentDir,
		AgentName:          "Test Agent",
	})

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.FeedbackProtocol, func(s netcore.Stream) {
		w.handleFeedbackStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.FeedbackProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	payload := map[string]string{
		"task":      "t1",
		"feedback":  "great work",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(s).Encode(payload)
	_ = s.CloseWrite()
	_, _ = io.ReadAll(s) // drain remote close
	_ = s.Close()
	<-done

	logPath := filepath.Join(agentDir, "feedback.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read feedback.log: %v", err)
	}
	if !strings.Contains(string(data), "great work") {
		t.Errorf("feedback.log missing body, got: %q", data)
	}
	if !strings.Contains(string(data), "t1") {
		t.Errorf("feedback.log missing task id, got: %q", data)
	}
}

// TestHandleFeedbackStream_InvalidJSON: malformed payload is silently
// dropped (stream Reset). Importantly, no feedback.log should be created.
func TestHandleFeedbackStream_InvalidJSON(t *testing.T) {
	agentDir := t.TempDir()
	w, workerHost := newTestWorker(t, Config{AgentDir: agentDir})

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.FeedbackProtocol, func(s netcore.Stream) {
		w.handleFeedbackStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.FeedbackProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_, _ = s.Write([]byte("{not json"))
	_ = s.CloseWrite()
	_, _ = io.ReadAll(s)
	_ = s.Close()
	<-done

	if _, err := os.Stat(filepath.Join(agentDir, "feedback.log")); !os.IsNotExist(err) {
		t.Errorf("feedback.log should not exist on decode failure, err=%v", err)
	}
}

// --- isDirEmpty ------------------------------------------------------------

// TestIsDirEmpty exercises the three branches of the helper: non-existent
// dir, empty dir, populated dir. It guards the decision between emitting
// [AGENTFM: FILES_INCOMING] vs [AGENTFM: NO_FILES] to the Boss.
func TestIsDirEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "nonexistent dir",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			// isDirEmpty returns false when it can't open the path. This is
			// intentional — a missing dir is treated as "has files" so we
			// err on the side of letting the caller handle it.
			want: false,
		},
		{
			name: "empty dir",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: true,
		},
		{
			name: "populated dir",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				_ = os.WriteFile(filepath.Join(d, "a.txt"), []byte("x"), 0o644)
				return d
			},
			want: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isDirEmpty(tc.setup(t))
			if got != tc.want {
				t.Errorf("isDirEmpty = %v, want %v", got, tc.want)
			}
		})
	}
}

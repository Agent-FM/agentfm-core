package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/internal/version"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// These tests cover the handler-side half of the "billed non-execution" fix.
// sandbox_outcome_test.go proves executePodman classifies outcomes correctly;
// what follows proves handleTaskStream ACTS on that classification.
//
// The distinction matters: without these, deleting the `if execErr == nil`
// guard before `status = metrics.StatusOK`, or the `reset = false` in the
// not-run branch, leaves the whole suite green — and silently restores the
// exact defect this work removed.

// taskCounter reads agentfm_tasks_total{status=<status>} out of the process
// registry. Returns 0 when the label tuple has not been touched yet.
//
// The registry is global to the test binary, so any test comparing these
// counters before/after MUST stay sequential — never call t.Parallel() in a
// test that drives handleTaskStream. The t.Setenv and t.Chdir calls below
// enforce this for the current tests (both panic under t.Parallel), but a
// future test without them would silently become flaky.
func taskCounter(t *testing.T, status string) float64 {
	t.Helper()
	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, f := range families {
		if f.GetName() != "agentfm_tasks_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "status" && l.GetValue() == status {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

// A worker whose sandbox cannot possibly start: PATH is emptied so `podman` is
// unresolvable and cmd.Start fails before any container exists.
func TestHandleTaskStream_NotRunIsNotASuccess(t *testing.T) {
	testutil.RequirePOSIX(t)
	t.Chdir(t.TempDir()) // executePodman anchors output on os.Getwd()

	w, workerHost := newTestWorker(t, Config{
		MaxConcurrentTasks: 1,
		MaxCPU:             90,
		MaxGPU:             90,
		ImageName:          "agentfm-test:v1",
		ModelName:          "test-model",
		Network:            NetworkHost,
		AgentDir:           t.TempDir(),
	})
	t.Setenv("PATH", t.TempDir()) // no podman anywhere on PATH

	beforeOK := taskCounter(t, metrics.StatusOK)
	beforeErr := taskCounter(t, metrics.StatusError)

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	payload, _ := json.Marshal(types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    "hello",
		TaskID:  "task-not-run",
	})
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = s.CloseWrite()

	// A graceful close is part of the contract: the Boss must be able to read
	// the failure text. A Reset here would surface as an error and lose it.
	raw, err := io.ReadAll(s)
	if err != nil {
		t.Fatalf("the worker reset the stream instead of closing it, so the Boss lost the reason: %v", err)
	}
	resp := string(raw)

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("handleTaskStream never returned")
	}

	if !strings.Contains(resp, "[AGENTFM: TASK_FAILED not_run]") {
		t.Errorf("no machine-readable failure marker for the Boss; got %q", resp)
	}
	if strings.Contains(resp, "[AGENTFM: NO_FILES]") {
		t.Errorf("a task that never ran must not report the normal no-artifact ending; got %q", resp)
	}

	if got := taskCounter(t, metrics.StatusOK); got != beforeOK {
		t.Errorf("agentfm_tasks_total{status=ok} moved on a task that never ran: %v -> %v", beforeOK, got)
	}
	if got := taskCounter(t, metrics.StatusError); got <= beforeErr {
		t.Errorf("agentfm_tasks_total{status=error} did not increment: %v -> %v", beforeErr, got)
	}
}

// An agent may write partial output and still exit non-zero. Those artifacts
// are legitimate and must still reach the Boss — while the task is recorded as
// a failure. This is the invariant that makes ErrSandboxFailed distinct from
// ErrSandboxNotRun rather than a single "it broke" error.
func TestHandleTaskStream_ArtifactsSurviveNonZeroExit(t *testing.T) {
	testutil.RequirePOSIX(t)
	t.Chdir(t.TempDir())

	w, workerHost := newTestWorker(t, Config{
		MaxConcurrentTasks: 1,
		MaxCPU:             90,
		MaxGPU:             90,
		ImageName:          "agentfm-test:v1",
		ModelName:          "test-model",
		Network:            NetworkHost,
		AgentDir:           t.TempDir(),
	})

	// Fake podman: on `run`, locate the -v host path (the argument ending in
	// ":/tmp/output:z"), drop a file in it, then exit non-zero. Any other
	// subcommand — notably the deferred `rm -f` — exits cleanly.
	testutil.InstallFakePodman(t, `#!/bin/sh
if [ "$1" = "run" ]; then
  for a in "$@"; do
    case "$a" in
      */tmp/output:z) echo partial > "${a%%:*}/artifact.txt" ;;
    esac
  done
  exit 3
fi
exit 0
`)

	beforeOK := taskCounter(t, metrics.StatusOK)
	beforeErr := taskCounter(t, metrics.StatusError)

	done := make(chan struct{})
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		w.handleTaskStream(context.Background(), s)
		close(done)
	})

	client := testutil.NewHost(t)

	// Capture the artifact payload rather than discarding it. Asserting only
	// on the [AGENTFM: FILES_INCOMING] marker would test that the worker
	// *announced* an artifact, not that one arrived — a broken ZipDirectory or
	// SendArtifacts would keep such a test green while the Boss received
	// nothing. handleTaskStream only slog.Errors a send failure, so the stream
	// is the sole observable.
	//
	// Wire format (internal/network/artifacts.go): int64 LE size | uint8
	// taskID length | taskID | zip bytes.
	gotZip := make(chan []byte, 1)
	client.SetStreamHandler(network.ArtifactProtocol, func(s netcore.Stream) {
		raw, _ := io.ReadAll(s)
		_ = s.Close()
		if len(raw) < 9 {
			gotZip <- nil
			return
		}
		idLen := int(raw[8])
		if len(raw) < 9+idLen {
			gotZip <- nil
			return
		}
		gotZip <- raw[9+idLen:]
	})
	testutil.ConnectHosts(t, client, workerHost)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, workerHost.ID(), network.TaskProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	payload, _ := json.Marshal(types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    "hello",
		TaskID:  "task-partial-artifacts",
	})
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = s.CloseWrite()

	raw, _ := io.ReadAll(s)
	resp := string(raw)

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("handleTaskStream never returned")
	}

	if !strings.Contains(resp, "[AGENTFM: FILES_INCOMING]") {
		t.Errorf("partial artifacts were dropped after a non-zero exit; got %q", resp)
	}
	if !strings.Contains(resp, "[AGENTFM: TASK_FAILED abnormal_exit]") {
		t.Errorf("the failure was not signalled to the Boss; got %q", resp)
	}

	// Marker ordering is part of the contract: FILES_INCOMING must precede
	// TASK_FAILED. A Boss-side reader that treats TASK_FAILED as terminal
	// would otherwise stop before the artifact announcement and drop exactly
	// the partial output this branch exists to preserve.
	failedAt := strings.Index(resp, "[AGENTFM: TASK_FAILED")
	filesAt := strings.Index(resp, "[AGENTFM: FILES_INCOMING]")
	if failedAt >= 0 && filesAt >= 0 && failedAt < filesAt {
		t.Errorf("TASK_FAILED was emitted before FILES_INCOMING; a Boss stopping at the failure marker would lose the artifacts:\n%q", resp)
	}

	// The artifact itself must actually reach the Boss.
	var zipped []byte
	select {
	case zipped = <-gotZip:
	case <-time.After(10 * time.Second):
		t.Fatal("no artifact stream ever reached the Boss")
	}
	zr, zerr := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if zerr != nil {
		t.Fatalf("artifact payload is not a valid zip: %v", zerr)
	}
	var found bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "artifact.txt") {
			found = true
		}
	}
	if !found {
		t.Error("the partial artifact written by the container is missing from the zip delivered to the Boss")
	}

	if got := taskCounter(t, metrics.StatusOK); got != beforeOK {
		t.Errorf("agentfm_tasks_total{status=ok} moved on a failed task: %v -> %v", beforeOK, got)
	}
	if got := taskCounter(t, metrics.StatusError); got <= beforeErr {
		t.Errorf("agentfm_tasks_total{status=error} did not increment: %v -> %v", beforeErr, got)
	}
}

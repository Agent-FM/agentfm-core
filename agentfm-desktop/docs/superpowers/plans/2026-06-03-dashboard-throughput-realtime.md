# Dashboard Throughput + Real-Time Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the desktop Dashboard count chat tasks (not just dispatch), gain "Assets built" + "Success rate" sparkline cards, and feel real-time via number tickers + sparkline pulse-dots + "updated Ns ago" header.

**Architecture:** Three small boss-side counter wiring changes in Go + four small renderer additions (one helper, two components, one route edit). No new endpoints, no new transports, polling stays at 2 s. Spec: `docs/superpowers/specs/2026-06-03-dashboard-throughput-realtime-design.md`.

**Tech Stack:** Go 1.x (`prometheus/client_golang`), React 18 + TypeScript, framer-motion, Tailwind, Vitest (unit), Playwright (e2e).

---

## File Structure

**Boss (Go) — `/Users/saif/Desktop/agentfm-prod/agentfm-core`:**
- Modify: `agentfm-go/internal/metrics/metrics.go` — declare + register new `ArtifactsBuiltTotal` counter.
- Modify: `agentfm-go/internal/network/artifacts.go` — increment `ArtifactsBuiltTotal` on the `success = true` line (artifact zip persisted to disk).
- Modify: `agentfm-go/internal/boss/openai_chat.go` — increment `TasksTotal` + observe `TaskDurationSeconds` from both `handleChatCompletions` (non-streaming) and `streamChatCompletion`.
- Test: `agentfm-go/internal/boss/openai_chat_metrics_test.go` (NEW) — assert the increments fire on success + error.
- Test: `agentfm-go/internal/network/artifacts_metrics_test.go` (NEW) — assert `ArtifactsBuiltTotal` fires on a clean handler run + does not fire on truncated/oversize/decode failures.

**Renderer (TypeScript) — `/Users/saif/Desktop/agentfm-prod/agentfm-desktop`:**
- Modify: `src/lib/metricsDerive.ts` — add `computeSuccessRateSeries(buffers)`.
- Modify: `tests/unit/metricsDerive.test.ts` — extend with `computeSuccessRateSeries` cases.
- Create: `src/components/dashboard/AnimatedNumber.tsx` — framer-motion count-up component.
- Create: `tests/unit/AnimatedNumber.test.tsx` — value-transitions + skip-on-big-jump.
- Modify: `src/components/charts/SparkLine.tsx` — overlay a `<span>` pulse-dot at the right edge above the canvas.
- Modify: `src/routes/Dashboard.tsx` — add Assets built tile, Success rate card, swap raw `<span>` headline values for `<AnimatedNumber>`, add 1 Hz "updated Ns ago" header.
- Modify: `tests/e2e/dashboard.spec.ts` — assert the new tile + card + header indicator render.

---

### Task 1: Add `ArtifactsBuiltTotal` counter on the boss

**Files:**
- Modify: `agentfm-core/agentfm-go/internal/metrics/metrics.go` (add var + register)

- [ ] **Step 1: Add the counter declaration**

In `agentfm-go/internal/metrics/metrics.go`, immediately after the `ArtifactBytesSentTotal` declaration (around line 111), add:

```go
// ArtifactsBuiltTotal counts artifact zips successfully received and
// persisted by this node. Useful for "assets built" dashboards where
// the byte counter doesn't tell the operator how many distinct
// deliverables landed.
var ArtifactsBuiltTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "agentfm_artifacts_built_total",
		Help: "Cumulative artifact zips received and persisted.",
	},
)
```

- [ ] **Step 2: Register it**

In the same file, find the `init()` function's `Registry.MustRegister(...)` call (around line 147). Insert `ArtifactsBuiltTotal,` between `ArtifactBytesSentTotal,` and `StreamErrorsTotal,`:

```go
func init() {
	Registry.MustRegister(
		TasksTotal,
		TaskDurationSeconds,
		WorkersOnline,
		ArtifactBytesSentTotal,
		ArtifactsBuiltTotal,
		StreamErrorsTotal,
		DHTQueriesTotal,
		AuthAttemptsTotal,
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
}
```

- [ ] **Step 3: Build check**

```bash
cd agentfm-core/agentfm-go && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
cd agentfm-core && git add agentfm-go/internal/metrics/metrics.go
git commit -m "feat(metrics): add ArtifactsBuiltTotal counter"
```

---

### Task 2: Increment `ArtifactsBuiltTotal` from the artifact handler (TDD)

**Files:**
- Test: `agentfm-core/agentfm-go/internal/network/artifacts_metrics_test.go` (NEW)
- Modify: `agentfm-core/agentfm-go/internal/network/artifacts.go:219`

- [ ] **Step 1: Write the failing test**

Create `agentfm-core/agentfm-go/internal/network/artifacts_metrics_test.go`:

```go
package network_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestArtifactsBuilt_IncrementsOnSuccess sends a small zip-shaped
// payload over the live ArtifactProtocol stream handler and asserts
// ArtifactsBuiltTotal incremented by exactly 1.
func TestArtifactsBuilt_IncrementsOnSuccess(t *testing.T) {
	// Sandbox CWD because HandleArtifactStream writes to
	// "./agentfm_artifacts/<id>.zip" relative to the process working
	// dir. Without this the test would scatter files into the repo.
	t.Chdir(t.TempDir())

	beforeCount := counterValue(t, metrics.ArtifactsBuiltTotal)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hosts := testutil.NewConnectedMesh(t, 2)
	server, client := hosts[0], hosts[1]
	server.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	payload := []byte("PK\x03\x04 fake-but-positive-bytes")
	taskID := "task_abc123"

	s, err := client.NewStream(ctx, server.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if err := s.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if err := binary.Write(s, binary.LittleEndian, int64(len(payload))); err != nil {
		t.Fatalf("write size: %v", err)
	}
	if err := binary.Write(s, binary.LittleEndian, uint8(len(taskID))); err != nil {
		t.Fatalf("write id len: %v", err)
	}
	if _, err := s.Write([]byte(taskID)); err != nil {
		t.Fatalf("write id: %v", err)
	}
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := s.CloseWrite(); err != nil {
		t.Fatalf("close-write: %v", err)
	}
	// Drain any handler response.
	_, _ = bytes.NewBuffer(nil).ReadFrom(s)
	_ = s.Close()

	// Allow the handler goroutine to finish.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if counterValue(t, metrics.ArtifactsBuiltTotal) > beforeCount {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	got := counterValue(t, metrics.ArtifactsBuiltTotal) - beforeCount
	if got != 1 {
		// Surface dir contents to make diagnosis easier.
		entries, _ := os.ReadDir(filepath.Join(".", "agentfm_artifacts"))
		t.Fatalf("expected ArtifactsBuiltTotal to grow by 1, grew by %v (artifacts dir: %v); server peer %s",
			got, entries, peer.ID(server.ID()).String())
	}
}

// TestArtifactsBuilt_DoesNotFireOnTruncated proves the increment is
// gated on the post-truncation success path: declaring a larger
// payload than is actually shipped must NOT mark the artifact as
// built.
func TestArtifactsBuilt_DoesNotFireOnTruncated(t *testing.T) {
	t.Chdir(t.TempDir())
	beforeCount := counterValue(t, metrics.ArtifactsBuiltTotal)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hosts := testutil.NewConnectedMesh(t, 2)
	server, client := hosts[0], hosts[1]
	server.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	declared := int64(200)
	short := []byte("PK\x03\x04 way-shorter-than-declared")
	taskID := "task_trunc"

	s, err := client.NewStream(ctx, server.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if err := s.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	_ = binary.Write(s, binary.LittleEndian, declared)
	_ = binary.Write(s, binary.LittleEndian, uint8(len(taskID)))
	_, _ = s.Write([]byte(taskID))
	_, _ = s.Write(short)
	_ = s.CloseWrite()
	_, _ = bytes.NewBuffer(nil).ReadFrom(s)
	_ = s.Close()

	// Give the handler a moment; it should NOT fire on a truncated payload.
	time.Sleep(500 * time.Millisecond)
	got := counterValue(t, metrics.ArtifactsBuiltTotal) - beforeCount
	if got != 0 {
		t.Fatalf("expected no increment on truncated artifact, got %v", got)
	}
}

func counterValue(t testing.TB, c prometheus.Counter) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	return m.GetCounter().GetValue()
}
```

- [ ] **Step 2: Run the test — expect failure**

```bash
cd agentfm-core/agentfm-go && go test ./internal/network/... -run TestArtifactsBuilt -v
```

Expected: `TestArtifactsBuilt_IncrementsOnSuccess` fails (`expected ArtifactsBuiltTotal to grow by 1, grew by 0`) because the handler doesn't yet increment the new counter. `TestArtifactsBuilt_DoesNotFireOnTruncated` passes vacuously.

- [ ] **Step 3: Add the increment**

In `agentfm-core/agentfm-go/internal/network/artifacts.go`, find the line `success = true` near the bottom of `HandleArtifactStream` (around line 219). Insert the counter increment IMMEDIATELY after it:

Before:
```go
	success = true
	pterm.Success.Printfln("🎉 Transfer Complete! Securely saved %d bytes to %s", bytesRead, destPath)
```

After:
```go
	success = true
	metrics.ArtifactsBuiltTotal.Inc()
	pterm.Success.Printfln("🎉 Transfer Complete! Securely saved %d bytes to %s", bytesRead, destPath)
```

- [ ] **Step 4: Run the tests — expect pass**

```bash
cd agentfm-core/agentfm-go && go test ./internal/network/... -run TestArtifactsBuilt -v -count=1
```

Expected: both pass.

- [ ] **Step 5: Commit**

```bash
cd agentfm-core
git add agentfm-go/internal/network/artifacts.go agentfm-go/internal/network/artifacts_metrics_test.go
git commit -m "feat(network): increment ArtifactsBuiltTotal on successful artifact persist"
```

---

### Task 3: Count chat completions in `TasksTotal` (TDD)

**Files:**
- Test: `agentfm-core/agentfm-go/internal/boss/openai_chat_metrics_test.go` (NEW)
- Modify: `agentfm-core/agentfm-go/internal/boss/openai_chat.go`

- [ ] **Step 1: Write the failing test**

Create `agentfm-core/agentfm-go/internal/boss/openai_chat_metrics_test.go`. Mirror the existing `openai_test.go` patterns — same package + same Boss-construction style.

```go
package boss

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestChatCompletion_IncrementsTasksTotalOnSuccess proves that a
// chat-completions request whose worker stream returns content
// causes TasksTotal{status=ok} to grow by exactly one.
//
// The dispatch path is already covered by execute.go's instrumentation
// — this test exists because the chat handler historically did not
// share that counter, which made the desktop Dashboard's "tasks"
// tile freeze for chat-only users.
func TestChatCompletion_IncrementsTasksTotalOnSuccess(t *testing.T) {
	b, worker := newBossWithStubWorker(t, /*streamReturns=*/[]byte("hello"))
	before := tasksCount(t, "ok")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"`+worker.PeerID+`","messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := tasksCount(t, "ok") - before; got != 1 {
		t.Fatalf("TasksTotal{ok}: grew by %v, want 1", got)
	}
}

// TestChatCompletion_IncrementsTasksTotalOnFailure proves the error
// path is also instrumented — worker stream error / timeout / dial
// failure should mark the task as error rather than silently skip.
func TestChatCompletion_IncrementsTasksTotalOnFailure(t *testing.T) {
	b, worker := newBossWithFailingWorker(t)
	before := tasksCount(t, "error")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"`+worker.PeerID+`","messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if got := tasksCount(t, "error") - before; got != 1 {
		t.Fatalf("TasksTotal{error}: grew by %v, want 1", got)
	}
}

// TestChatCompletion_StreamingIncrementsTasksTotalOnSuccess covers
// the SSE branch separately because it shares no code with the
// non-streaming branch beyond the dispatch helpers.
func TestChatCompletion_StreamingIncrementsTasksTotalOnSuccess(t *testing.T) {
	b, worker := newBossWithStubWorker(t, []byte("hello"))
	before := tasksCount(t, "ok")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"`+worker.PeerID+`","stream":true,"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	// SSE handler keeps the connection open until the worker stream
	// closes; the stub closes synchronously so the call returns.
	if got := tasksCount(t, "ok") - before; got != 1 {
		t.Fatalf("TasksTotal{ok} (stream): grew by %v, want 1", got)
	}
}

func tasksCount(t testing.TB, status string) float64 {
	t.Helper()
	c, err := metrics.TasksTotal.GetMetricWithLabelValues(status)
	if err != nil {
		t.Fatalf("TasksTotal lookup: %v", err)
	}
	m := &dto.Metric{}
	if err := c.(prometheus.Counter).Write(m); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	return m.GetCounter().GetValue()
}

// newBossWithStubWorker — see the existing openai_test.go helpers for
// the canonical setup. Replicate the same pattern (real TCP host, in-
// process boss, stub worker stream returning streamReturns then EOF).
// If a comparable helper already exists in openai_test.go, reuse it
// here via a wrapper in this file rather than duplicating; otherwise
// copy the helper verbatim.
func newBossWithStubWorker(t *testing.T, streamReturns []byte) (*Boss, struct{ PeerID string }) {
	t.Helper()
	// IMPORTANT: this helper mirrors the existing openai_test.go
	// fixture. Read openai_test.go before this task and copy that
	// fixture's body — do NOT invent a new one.
	t.Fatalf("UNIMPLEMENTED: copy the stub-boss-with-worker helper from openai_test.go")
	return nil, struct{ PeerID string }{}
}

func newBossWithFailingWorker(t *testing.T) (*Boss, struct{ PeerID string }) {
	t.Helper()
	t.Fatalf("UNIMPLEMENTED: copy the stub-boss-with-failing-worker helper from openai_test.go")
	return nil, struct{ PeerID string }{}
}
```

Now read `internal/boss/openai_test.go` to find the existing test fixtures (the file has working stub Boss + stub worker setups). Replace the two `t.Fatalf("UNIMPLEMENTED: …")` helpers in your new file with the body of the equivalent helpers from that file. If you can extract them into shared `testing.TB` helpers in a `*_test.go` file in the same package, do so (Go's `internal/boss` package allows shared test helpers). If they're inline in test cases, copy them inline.

- [ ] **Step 2: Run the tests — expect failure**

```bash
cd agentfm-core/agentfm-go && go test ./internal/boss/... -run TestChatCompletion_Increments -v
```

Expected: tests fail because the chat handlers don't yet increment `TasksTotal`. (After replacing the `UNIMPLEMENTED` helpers, the tests should at least compile and run, then fail on the assertion.)

- [ ] **Step 3: Instrument the non-streaming chat handler**

In `agentfm-core/agentfm-go/internal/boss/openai_chat.go`, modify `handleChatCompletions`. Add a start timestamp at the top of the request handling (after the request-parsing guards return), and instrument the existing terminal points:

Current `handleChatCompletions` (lines 17-100). After the `if req.Stream` branch returns control (the streaming case is instrumented separately in Step 4) and before the synchronous-path `ts := b.openTaskStream(...)` call, capture a start time:

```go
	prompt := renderChatPrompt(req.Messages)
	taskID := newCompletionID("task_")

	if req.Stream {
		b.streamChatCompletion(r.Context(), w, peerID, req.Model, prompt, taskID)
		return
	}

	start := time.Now()
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	ts := b.openTaskStream(r.Context(), w, peerID, prompt, taskID)
	if ts == nil {
		metrics.TasksTotal.WithLabelValues(metrics.StatusError).Inc()
		if b.completionRater != nil {
			b.completionRater.RecordOutcome(peerID, OutcomeFailure)
		}
		return
	}
	defer func() {
		ts.close()
		status := metrics.StatusError
		if ts.success {
			status = metrics.StatusOK
		}
		metrics.TasksTotal.WithLabelValues(status).Inc()
		if b.completionRater != nil {
			if ts.success {
				b.completionRater.RecordOutcome(peerID, OutcomeSuccess)
			} else {
				b.completionRater.RecordOutcome(peerID, OutcomeFailure)
			}
		}
	}()
```

You will need to add `"agentfm/internal/metrics"` and `"time"` to the existing imports if not already present.

- [ ] **Step 4: Instrument the streaming chat handler**

In the same file, find `streamChatCompletion` (starts around line 102). At the top, capture `start := time.Now()` and add a deferred `metrics.TaskDurationSeconds.Observe(time.Since(start).Seconds())`. In the existing `defer func() { ts.close(); ... }()` block (currently records reputation via `b.completionRater.RecordOutcome`), additionally call `metrics.TasksTotal.WithLabelValues(status).Inc()` where `status = metrics.StatusOK` if `ts.success` else `metrics.StatusError`. For the early-return branch `if ts == nil` (lines 104-109), also call `metrics.TasksTotal.WithLabelValues(metrics.StatusError).Inc()` before returning.

Resulting top of `streamChatCompletion`:

```go
func (b *Boss) streamChatCompletion(ctx context.Context, w http.ResponseWriter, peerID peer.ID, model, prompt, taskID string) {
	start := time.Now()
	defer func() {
		metrics.TaskDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	ts := b.openTaskStream(ctx, w, peerID, prompt, taskID)
	if ts == nil {
		metrics.TasksTotal.WithLabelValues(metrics.StatusError).Inc()
		if b.completionRater != nil {
			b.completionRater.RecordOutcome(peerID, OutcomeFailure)
		}
		return
	}
	defer func() {
		ts.close()
		status := metrics.StatusError
		if ts.success {
			status = metrics.StatusOK
		}
		metrics.TasksTotal.WithLabelValues(status).Inc()
		if b.completionRater != nil {
			if ts.success {
				b.completionRater.RecordOutcome(peerID, OutcomeSuccess)
			} else {
				b.completionRater.RecordOutcome(peerID, OutcomeFailure)
			}
		}
	}()
```

- [ ] **Step 5: Run the tests — expect pass**

```bash
cd agentfm-core/agentfm-go && go test ./internal/boss/... -run TestChatCompletion -v -count=1
```

Expected: all three tests pass.

- [ ] **Step 6: Run the full boss test suite to check for regression**

```bash
cd agentfm-core/agentfm-go && go test ./internal/boss/... -count=1
```

Expected: all green.

- [ ] **Step 7: Commit**

```bash
cd agentfm-core
git add agentfm-go/internal/boss/openai_chat.go agentfm-go/internal/boss/openai_chat_metrics_test.go
git commit -m "feat(boss): chat completions count toward TasksTotal + TaskDurationSeconds"
```

---

### Task 4: Add `computeSuccessRateSeries` helper (TDD)

**Files:**
- Modify: `agentfm-desktop/src/lib/metricsDerive.ts`
- Modify: `agentfm-desktop/tests/unit/metricsDerive.test.ts`

- [ ] **Step 1: Write failing tests**

Open `agentfm-desktop/tests/unit/metricsDerive.test.ts` and append:

```ts
import { computeSuccessRateSeries } from '../../src/lib/metricsDerive'
import { createRingBuffer, pushSample, type RingBuffer } from '../../src/types/metrics'
import { seriesKey } from '../../src/lib/metricsStore'

function makeBuf(values: number[]): RingBuffer {
  const buf = createRingBuffer()
  let t = 1_000_000
  for (const v of values) {
    pushSample(buf, t, v)
    t += 2000
  }
  return buf
}

describe('computeSuccessRateSeries', () => {
  it('returns [] when no OK buffer exists', () => {
    const map = new Map<string, RingBuffer>()
    expect(computeSuccessRateSeries(map)).toEqual([])
  })

  it('returns all 1.0 when traffic is all OK', () => {
    const map = new Map<string, RingBuffer>()
    map.set(seriesKey('agentfm_tasks_total', { status: 'ok' }), makeBuf([0, 1, 2, 3]))
    const got = computeSuccessRateSeries(map)
    expect(got).toEqual([1, 1, 1, 1])
  })

  it('returns all 0.0 when traffic is all error', () => {
    const map = new Map<string, RingBuffer>()
    map.set(seriesKey('agentfm_tasks_total', { status: 'ok' }), makeBuf([0, 0, 0, 0]))
    map.set(seriesKey('agentfm_tasks_total', { status: 'error' }), makeBuf([0, 1, 2, 3]))
    const got = computeSuccessRateSeries(map)
    expect(got).toEqual([1, 0, 0, 0]) // first sample has 0/0 → 1.0
  })

  it('mixed series returns the expected ratio at each tick', () => {
    const map = new Map<string, RingBuffer>()
    map.set(seriesKey('agentfm_tasks_total', { status: 'ok' }), makeBuf([0, 3, 6]))
    map.set(seriesKey('agentfm_tasks_total', { status: 'error' }), makeBuf([0, 1, 2]))
    const got = computeSuccessRateSeries(map)
    expect(got[0]).toBe(1)
    expect(got[1]).toBeCloseTo(3 / 4, 5)
    expect(got[2]).toBeCloseTo(6 / 8, 5)
  })

  it('returns 1.0 for ticks with zero denominator', () => {
    const map = new Map<string, RingBuffer>()
    map.set(seriesKey('agentfm_tasks_total', { status: 'ok' }), makeBuf([0, 0]))
    const got = computeSuccessRateSeries(map)
    expect(got).toEqual([1, 1])
  })
})
```

- [ ] **Step 2: Run the test — expect failure**

```bash
cd agentfm-desktop && npx vitest run tests/unit/metricsDerive.test.ts -t computeSuccessRateSeries
```

Expected: 5 failures, all due to `computeSuccessRateSeries is not a function`.

- [ ] **Step 3: Implement the helper**

In `agentfm-desktop/src/lib/metricsDerive.ts`, append:

```ts
import { seriesKey } from './metricsStore'
import { type RingBuffer, ringToArrays } from '../types/metrics'

const TASK_STATUSES = ['ok', 'error', 'rejected', 'timeout'] as const

/**
 * Computes a series of point-in-time success rates from the four
 * `agentfm_tasks_total{status}` ring buffers. At each timestamp the
 * value is `ok / (ok + error + rejected + timeout)`. Ratio is 1.0
 * when the denominator is zero (treat "no traffic yet" as "no failures").
 *
 * Aligns by index — all four series share the same poll cadence
 * because they're scraped from the same /metrics fetch. A missing
 * status buffer contributes 0 to the denominator.
 *
 * Returns an empty array when no OK buffer exists at all (the
 * dashboard then renders nothing for the sparkline).
 */
export function computeSuccessRateSeries(
  buffers: Map<string, RingBuffer>,
): number[] {
  const okBuf = buffers.get(seriesKey('agentfm_tasks_total', { status: 'ok' }))
  if (!okBuf) return []
  const okValues = ringToArrays(okBuf).v
  if (okValues.length === 0) return []

  const otherValues = TASK_STATUSES.filter((s) => s !== 'ok').map((status) => {
    const buf = buffers.get(seriesKey('agentfm_tasks_total', { status }))
    if (!buf) return new Array<number>(okValues.length).fill(0)
    return ringToArrays(buf).v
  })

  const out: number[] = []
  for (let i = 0; i < okValues.length; i++) {
    const ok = okValues[i] ?? 0
    let other = 0
    for (const series of otherValues) {
      other += series[i] ?? 0
    }
    const denom = ok + other
    out.push(denom === 0 ? 1 : ok / denom)
  }
  return out
}
```

If `metricsDerive.ts` already imports `seriesKey` or the `types/metrics` symbols, do not double-import — fold the new symbol into the existing import statements.

- [ ] **Step 4: Run the test — expect pass**

```bash
cd agentfm-desktop && npx vitest run tests/unit/metricsDerive.test.ts
```

Expected: all green (the old tests in the file should continue to pass).

- [ ] **Step 5: Commit**

```bash
cd agentfm-desktop
git add src/lib/metricsDerive.ts tests/unit/metricsDerive.test.ts
git commit -m "feat(metrics): computeSuccessRateSeries helper for dashboard"
```

---

### Task 5: Add `AnimatedNumber` component (TDD)

**Files:**
- Create: `agentfm-desktop/src/components/dashboard/AnimatedNumber.tsx`
- Create: `agentfm-desktop/tests/unit/AnimatedNumber.test.tsx`

- [ ] **Step 1: Write the failing tests**

Create `agentfm-desktop/tests/unit/AnimatedNumber.test.tsx`:

```tsx
import { render, act } from '@testing-library/react'
import { AnimatedNumber } from '../../src/components/dashboard/AnimatedNumber'

// Vitest's fake timers integrate with framer-motion's frame loop
// through window.requestAnimationFrame, which our manual frame ticks
// drive forward.
function tick(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms)
  })
}

describe('AnimatedNumber', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the initial value immediately', () => {
    const { container } = render(<AnimatedNumber value={42} />)
    expect(container.textContent).toBe('42')
  })

  it('tweens between values when changed by less than 10x', () => {
    const { container, rerender } = render(<AnimatedNumber value={10} />)
    expect(container.textContent).toBe('10')

    rerender(<AnimatedNumber value={100} />)
    // Halfway through the 500ms tween, the displayed number should be
    // strictly between 10 and 100.
    tick(250)
    const mid = Number(container.textContent)
    expect(mid).toBeGreaterThan(10)
    expect(mid).toBeLessThan(100)

    tick(500)
    expect(container.textContent).toBe('100')
  })

  it('skips the tween when the jump is >10x', () => {
    const { container, rerender } = render(<AnimatedNumber value={5} />)
    rerender(<AnimatedNumber value={1000} />)
    tick(16) // one frame
    expect(container.textContent).toBe('1000')
  })

  it('respects a custom format prop', () => {
    const { container } = render(
      <AnimatedNumber value={0.842} format={(n) => `${(n * 100).toFixed(1)}%`} />,
    )
    expect(container.textContent).toBe('84.2%')
  })
})
```

- [ ] **Step 2: Run the test — expect failure**

```bash
cd agentfm-desktop && npx vitest run tests/unit/AnimatedNumber.test.tsx
```

Expected: fails because the file doesn't exist.

- [ ] **Step 3: Create the component**

Create `agentfm-desktop/src/components/dashboard/AnimatedNumber.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { animate } from 'framer-motion'

const TWEEN_MS = 500
const SKIP_RATIO = 10

interface Props {
  value: number
  format?: (n: number) => string
}

/**
 * Tweens its text content from the previous value to the new one over
 * 500 ms when the value changes. Skips the tween (snaps instantly) if
 * the value changes by more than SKIP_RATIO× — that case is almost
 * always a boss restart resetting counters, and a long visible
 * count-down to zero is more confusing than useful.
 *
 * Uses framer-motion's `animate(from, to, opts)` which drives a
 * rAF-based tween without mounting a React subtree.
 */
export function AnimatedNumber({ value, format }: Props) {
  const [display, setDisplay] = useState(value)
  const lastValueRef = useRef(value)

  useEffect(() => {
    const prev = lastValueRef.current
    lastValueRef.current = value
    if (prev === value) return

    const big =
      Math.abs(value - prev) > Math.max(Math.abs(prev), 1) * SKIP_RATIO
    if (big) {
      setDisplay(value)
      return
    }
    const controls = animate(prev, value, {
      duration: TWEEN_MS / 1000,
      ease: 'easeOut',
      onUpdate: (v) => setDisplay(v),
    })
    return () => controls.stop()
  }, [value])

  const fmt = format ?? ((n: number) => Math.round(n).toString())
  return <span>{fmt(display)}</span>
}
```

- [ ] **Step 4: Run the test — expect pass**

```bash
cd agentfm-desktop && npx vitest run tests/unit/AnimatedNumber.test.tsx
```

Expected: all 4 cases pass. If the tween-progress assertion fails because framer-motion's frame timer doesn't advance under fake timers, fall back to advancing real time with `await new Promise(r => setTimeout(r, 250))` and switch the test to use real timers.

- [ ] **Step 5: Commit**

```bash
cd agentfm-desktop
git add src/components/dashboard/AnimatedNumber.tsx tests/unit/AnimatedNumber.test.tsx
git commit -m "feat(dashboard): AnimatedNumber tween component"
```

---

### Task 6: Add pulse-dot tip to SparkLine

**Files:**
- Modify: `agentfm-desktop/src/components/charts/SparkLine.tsx`

The existing `SparkLine` renders a `<canvas>`. The pulse-dot must be an overlaid HTML element so we can use the existing `animate-pulse-cyan` tailwind keyframe.

- [ ] **Step 1: Wrap the canvas + add the dot**

Replace the body of `SparkLine.tsx` with:

```tsx
import { useEffect, useRef, useState } from 'react'

export interface SparkLineProps {
  values: number[]
  width: number
  height: number
  color: string
}

export function SparkLine({ values, width, height, color }: SparkLineProps) {
  const ref = useRef<HTMLCanvasElement>(null)
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null)

  useEffect(() => {
    const canvas = ref.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    canvas.width = width * dpr
    canvas.height = height * dpr
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, width, height)

    if (values.length === 0) {
      setTip(null)
      return
    }

    let min = values[0]
    let max = values[0]
    for (const v of values) {
      if (v < min) min = v
      if (v > max) max = v
    }
    const range = max - min || 1
    const stepX = values.length > 1 ? width / (values.length - 1) : width

    ctx.beginPath()
    let lastX = 0
    let lastY = 0
    for (let i = 0; i < values.length; i++) {
      const x = i * stepX
      const y = height - ((values[i] - min) / range) * (height - 2) - 1
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
      lastX = x
      lastY = y
    }
    ctx.strokeStyle = color
    ctx.lineWidth = 1.2
    ctx.stroke()

    ctx.lineTo(width, height)
    ctx.lineTo(0, height)
    ctx.closePath()
    ctx.fillStyle = color + '22'
    ctx.fill()

    setTip(values.length >= 2 ? { x: lastX, y: lastY } : null)
  }, [values, width, height, color])

  return (
    <div className="relative" style={{ width, height }}>
      <canvas ref={ref} />
      {tip && (
        <span
          aria-hidden
          className="absolute w-1.5 h-1.5 rounded-full animate-pulse-cyan pointer-events-none"
          style={{
            background: color,
            boxShadow: `0 0 6px ${color}`,
            left: tip.x - 3,
            top: tip.y - 3,
          }}
        />
      )}
    </div>
  )
}
```

- [ ] **Step 2: Run existing tests to verify no regression**

```bash
cd agentfm-desktop && npx vitest run --reporter=verbose 2>&1 | tail -30
```

Expected: all green. SparkLine itself has no unit tests; the visual change is verified in Task 8 via Playwright + manual inspection.

- [ ] **Step 3: Commit**

```bash
cd agentfm-desktop
git add src/components/charts/SparkLine.tsx
git commit -m "feat(charts): pulse-dot tip on SparkLine to signal liveness"
```

---

### Task 7: Wire the new tiles + tickers into the Dashboard route

**Files:**
- Modify: `agentfm-desktop/src/routes/Dashboard.tsx`

- [ ] **Step 1: Add imports**

At the top of `Dashboard.tsx`, add to the existing import block:

```ts
import { useEffect, useMemo, useState } from 'react'
import { computeSuccessRateSeries } from '../lib/metricsDerive'
import { AnimatedNumber } from '../components/dashboard/AnimatedNumber'
```

Remove `useMemo`-only and `useEffect`-only re-imports if they were separate lines.

- [ ] **Step 2: Add new selectors near the existing ones**

After the existing `errorsByProtocol` `useMemo` (around line 105-115), add:

```ts
const assetsBuiltBuf = bossSeries.get(seriesKey('agentfm_artifacts_built_total', {}))
const assetsBuiltCount = latestValue(assetsBuiltBuf ?? emptyBuf()) ?? 0
const assetsBuiltValues = assetsBuiltBuf ? ringToArrays(assetsBuiltBuf).v : []

const successRateSeries = useMemo(
  () => computeSuccessRateSeries(bossSeries),
  [bossSeries],
)
const successRateNow = successRateSeries.length > 0
  ? (successRateSeries[successRateSeries.length - 1] ?? 1)
  : 1

const successRateColor =
  successRateNow >= 0.95 ? '#84cc16'
  : successRateNow >= 0.80 ? '#fbbf24'
  : '#f43f5e'
```

- [ ] **Step 3: Tick the "updated Ns ago" header at 1 Hz**

Inside the `Dashboard` component (near the top), add:

```ts
const [, forceTick] = useState(0)
useEffect(() => {
  const id = setInterval(() => forceTick((n) => n + 1), 1000)
  return () => clearInterval(id)
}, [])
```

In the header `<div className="flex justify-between items-center">` block, replace the existing stale-only span with:

```tsx
<div className={`text-[11px] font-mono ${stale ? 'text-warn' : 'text-text-2'}`}>
  {lastTick === 0
    ? 'connecting…'
    : `updated ${(staleAgeMs / 1000).toFixed(1)}s ago`}
</div>
```

(That replaces the conditional `{stale && (...)}` rendering.)

- [ ] **Step 4: Animate the headline numbers**

In the tasks-hero card (`<NeonCard className="p-5 mb-4">`), replace:

```tsx
<div className="text-[40px] font-mono font-bold text-accent leading-none mb-3">
  {Math.round(totalTasks)}
</div>
```

with:

```tsx
<div className="text-[40px] font-mono font-bold text-accent leading-none mb-3">
  <AnimatedNumber value={totalTasks} />
</div>
```

In the "Agents online" `<Tile>`, change `value={\`${Math.round(workersOnline)}\`}` to:

```tsx
<Tile label="Agents online" value={<AnimatedNumber value={workersOnline} />} color="#a855f7" />
```

To make this work, update the `Tile` component (at the bottom of the file) to accept `value: ReactNode` instead of `value: string`:

```tsx
import type { ReactNode } from 'react'

function Tile({
  label,
  value,
  hint,
  color,
}: {
  label: string
  value: ReactNode
  hint?: string
  color: string
}) {
  return (
    <NeonCard className="p-4">
      <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
        {label}
      </div>
      <div className="font-mono font-bold leading-none" style={{ fontSize: 28, color }}>
        {value}
      </div>
      {hint && (
        <div className="text-[11px] text-text-2 mt-2">{hint}</div>
      )}
    </NeonCard>
  )
}
```

- [ ] **Step 5: Add the Assets built tile**

Replace the existing 3-column grid containing "Output speed / Sign-in attempts / Errors by channel":

```tsx
<div className="grid grid-cols-3 gap-3.5 mb-4">
```

with a 4-column grid that adds Assets built before the Errors-by-channel card:

```tsx
<div className="grid grid-cols-4 gap-3.5 mb-4">
  <Tile
    label="Output speed"
    value={`${(artifactBytesPerSec / 1024).toFixed(1)} KB/s`}
    hint="files sent back from agents"
    color="#22d3ee"
  />
  <Tile label="Sign-in attempts" value={`${Math.round(authAttemptsTotal)}`} color="#a855f7" />
  <NeonCard className="p-4">
    <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
      Assets built
    </div>
    <div className="font-mono font-bold leading-none mb-2" style={{ fontSize: 28, color: '#22d3ee' }}>
      <AnimatedNumber value={assetsBuiltCount} />
    </div>
    <SparkLine values={assetsBuiltValues} width={180} height={28} color="#22d3ee" />
  </NeonCard>
  <NeonCard className="p-4">
    {/* existing Errors by channel content — keep verbatim */}
  </NeonCard>
</div>
```

(Copy the existing `Errors by channel` `NeonCard` block contents into the second `NeonCard` slot above. Do not duplicate the existing wrapping `NeonCard`.)

- [ ] **Step 6: Add the Success rate card**

Immediately after the 4-column grid you just created, add:

```tsx
<NeonCard className="p-5 mb-4">
  <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
    Success rate · last 2 min
  </div>
  <div
    className="text-[40px] font-mono font-bold leading-none mb-3"
    style={{ color: successRateColor }}
  >
    <AnimatedNumber
      value={successRateNow}
      format={(n) => `${(n * 100).toFixed(1)}%`}
    />
  </div>
  <SparkLine
    values={successRateSeries.map((r) => r * 100)}
    width={800}
    height={40}
    color={successRateColor}
  />
</NeonCard>
```

- [ ] **Step 7: Typecheck + build**

```bash
cd agentfm-desktop && npx tsc --noEmit --project tsconfig.web.json 2>&1 | grep -E "Dashboard|AnimatedNumber|metricsDerive|SparkLine" | head -10
```

Expected: no errors specific to the files this task touches.

```bash
cd agentfm-desktop && npm run build 2>&1 | tail -5
```

Expected: builds cleanly.

- [ ] **Step 8: Commit**

```bash
cd agentfm-desktop
git add src/routes/Dashboard.tsx
git commit -m "feat(dashboard): assets built + success rate cards, animated tickers, freshness header"
```

---

### Task 8: Extend the dashboard e2e test

**Files:**
- Modify: `agentfm-desktop/tests/e2e/dashboard.spec.ts`

- [ ] **Step 1: Add assertions for the new UI**

Open `tests/e2e/dashboard.spec.ts`. After the existing assertion that the TASKS section renders, add:

```ts
test('dashboard surfaces assets built tile, success rate card, and freshness header', async () => {
  // The Dashboard route mounts at #/dashboard; the existing test in
  // this file already navigates there.
  await expect(page.locator('text=Assets built').first()).toBeVisible({ timeout: 10_000 })
  await expect(page.locator('text=Success rate').first()).toBeVisible()
  // Freshness banner is the only element that says "updated" + number + "s ago"
  // or "connecting…" before the first poll completes.
  await expect(
    page.locator('text=/updated [\\d.]+s ago|connecting…/').first(),
  ).toBeVisible({ timeout: 10_000 })
})
```

If the file already has a global `beforeAll` that opens the dashboard via `Cmd+5` / clicking `Dashboard`, you don't need to navigate again. If it doesn't, add a `await page.locator('a[href="#/dashboard"]').click()` before the assertions.

- [ ] **Step 2: Run the e2e suite**

```bash
cd agentfm-desktop && pkill -f "agentfm -mode api -apiport 8080" 2>/dev/null; sleep 2
npx playwright test dashboard --reporter=line 2>&1 | tail -15
```

Expected: all green.

- [ ] **Step 3: Commit**

```bash
cd agentfm-desktop
git add tests/e2e/dashboard.spec.ts
git commit -m "test(e2e): dashboard renders assets built + success rate + freshness"
```

---

### Task 9: End-to-end smoke + race-suite gate

**Files:** none modified.

- [ ] **Step 1: Boss-side race tests**

```bash
cd agentfm-core/agentfm-go && make test-race 2>&1 | tail -25
```

Expected: every package `ok`, no race detector warnings. Per CLAUDE.md this is the project's PR gate.

- [ ] **Step 2: Renderer build + unit + e2e**

```bash
cd agentfm-desktop
npm run build 2>&1 | tail -3
npx vitest run 2>&1 | tail -5
pkill -f "agentfm -mode api -apiport 8080" 2>/dev/null; sleep 2
npx playwright test --reporter=line 2>&1 | tail -10
```

Expected: build clean, all unit green, all e2e green.

- [ ] **Step 3: Manual smoke**

1. Start the desktop dev server: `cd agentfm-desktop && npm run dev` (in a separate terminal).
2. Switch to the **Dashboard** tab (Cmd+5).
3. Verify the header now reads `updated 0.0s ago` and the digit ticks up every second.
4. Verify the pulse-dot at the right edge of each sparkline pulses.
5. With the desktop's worker `SickLeave-pub` already online from prior testing, dispatch a sick-leave task from the radar. The "Tasks · last 5 min" number should animate up, and "Assets built" should tick by 1 after the artifact lands.
6. In the chat tab, pin `SickLeave-pub`, send "flu, ooo today", let it complete. The dashboard's task count should now move (this is the chat-fix). Without the fix the dashboard would stay flat.

- [ ] **Step 4: No commit — this is verification only.**

---

## Self-Review

**Spec coverage check:**

- Boss-side §1 ArtifactsBuiltTotal counter → Task 1. ✓
- Boss-side §2 increment site → Task 2 (TDD). ✓
- Boss-side §3 chat handler instrumentation (streaming + non-streaming) → Task 3 (TDD with three tests). ✓
- Renderer §4 selectors → Task 7 (Step 2). ✓
- Renderer §5 `computeSuccessRateSeries` helper → Task 4 (TDD). ✓
- Renderer §6 tile renders → Task 7 (Steps 5-6). ✓
- Renderer §7 AnimatedNumber → Task 5 (TDD). ✓
- Renderer §8 SparkLine pulse-dot tip → Task 6. ✓
- Renderer §9 updated Ns ago header → Task 7 (Step 3). ✓
- Spec testing §boss → Task 2 + Task 3 (TDD). ✓
- Spec testing §renderer unit → Task 4 + Task 5 (TDD). ✓
- Spec testing §renderer e2e → Task 8. ✓

**Placeholder scan:** Task 3 Step 1 deliberately ships `t.Fatalf("UNIMPLEMENTED: …")` in the two test-helper functions with explicit instructions to read `internal/boss/openai_test.go` and copy the existing helpers. This is intentional honesty — the exact helper body is in code the plan-writer hasn't quoted, and inventing a fake one would be worse than pointing to the source of truth. The implementer must read `openai_test.go` before Step 2 of Task 3. Every other step ships complete code.

**Type consistency:**
- `computeSuccessRateSeries(buffers: Map<string, RingBuffer>): number[]` — same signature in Task 4 and Task 7.
- `AnimatedNumber({ value, format })` — same prop shape in Tasks 5 and 7.
- `successRateColor` is computed once in Task 7 Step 2 and consumed in Step 6; no naming drift.
- `Tile` component prop change from `value: string` to `value: ReactNode` in Task 7 Step 4 is explicit, with the new signature shown.
- Go: `metrics.ArtifactsBuiltTotal` (Task 1), `metrics.TasksTotal` (Task 3), `metrics.TaskDurationSeconds` (Task 3), `metrics.StatusOK` / `metrics.StatusError` (Task 3) — all match the existing `metrics.go` exports.

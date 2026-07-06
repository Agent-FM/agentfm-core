# Dashboard Throughput + Real-Time Polish — Design

**Status:** Approved — ready for implementation plan.
**Branches affected:** `agentfm-core/54_desktop_app` (Go boss) + `agentfm-desktop/metrics-dashboard` (renderer).

## Problem

The desktop Dashboard route (`src/routes/Dashboard.tsx`) polls the boss `/metrics` endpoint every 2 s and renders the result. Two complaints:

1. **Chat tasks don't count.** The `agentfm_tasks_total` counter only moves for the dispatch code paths (`internal/boss/execute.go:117`, `internal/boss/api_handlers.go:175`, `internal/boss/api_async.go:161`). The chat-completions handler in `internal/boss/openai_chat.go` records reputation outcomes via `b.completionRater.RecordOutcome(...)` but never increments `metrics.TasksTotal`. Result: a user who only uses the chat surface sees a frozen "0 tasks" Dashboard tile even though real work is happening.

2. **No throughput insight beyond "tasks ok".** The Dashboard surfaces task counts, p95 latency, workers online, stream errors, artifact bytes/s, auth attempts, and process health — but nothing for assets-built count, no success-rate trend, no sense of motion. The user feels the dashboard is stale.

3. **No visual "liveness" cue.** Polling is 2 s and the numbers tick over instantly. The user can't tell at a glance whether the dashboard is still polling — that ambiguity is part of the "not real-time" complaint.

## Out of scope

Explicitly rejected during brainstorming:
- Switching the transport from polling to SSE / WebSocket. The 2 s cadence is fine; the complaint is visual, not transport.
- Per-worker task breakdown (would require a `worker` label on `TasksTotal`, cardinality + scope creep).
- Latency p50/p95/p99 sparklines (category B from brainstorming).
- Reputation / equivocator dashboards (category C).
- Active-dispatch gauge + recent-task timeline (category D).
- Dashboard preferences (animation toggle, time-window selector).

## Architecture

```
┌──────────────────────────── boss (Go) ────────────────────────────┐
│                                                                    │
│  openai_chat.go                                                    │
│   ├─ streamChatCompletion        → TasksTotal.Inc(status)  (NEW)   │
│   └─ handleChatCompletions       → TasksTotal.Inc(status)  (NEW)   │
│                                  → TaskDurationSeconds.Observe (NEW)│
│                                                                    │
│  api_artifacts.go (artifact receive handler)                       │
│   └─ on successful zip persist   → ArtifactsBuiltTotal.Inc (NEW)   │
│                                                                    │
│  metrics/metrics.go                                                │
│   └─ ArtifactsBuiltTotal — new prometheus.Counter         (NEW)    │
│                                                                    │
│  /metrics endpoint serves all above (unchanged).                   │
└────────────────────────────────────────────────────────────────────┘
                                │
                                │ HTTP GET every 2 s
                                ▼
┌─────────────────────── desktop renderer ───────────────────────────┐
│                                                                    │
│  useMetricsPoll  (unchanged — 2 s interval, visibility-paused,     │
│                   3-strike backoff)                                │
│        │                                                           │
│        ▼                                                           │
│  parseMetrics → metricsStore (ring buffers per series)             │
│        │                                                           │
│        ▼                                                           │
│  Dashboard.tsx selectors:                                          │
│   • existing: total tasks, p95, workers online, errors, bytes/s    │
│   • NEW: assetsBuiltCount, assetsBuiltSeries                       │
│   • NEW: successRateNow, successRateSeries                         │
│        │                                                           │
│        ▼                                                           │
│  Render:                                                           │
│   • motion.span tickers on headline tiles  (NEW)                   │
│   • SparkLine + pulse-dot tip              (NEW)                   │
│   • "updated Ns ago" header                (NEW, replaces stale)   │
└────────────────────────────────────────────────────────────────────┘
```

No new transport, no new endpoints, no new IPC. Existing 2 s poll feeds everything.

## Components

### Boss side

#### 1. `metrics.ArtifactsBuiltTotal`

In `internal/metrics/metrics.go`, after `ArtifactBytesSentTotal`:

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

Add `ArtifactsBuiltTotal` to the `Registry.MustRegister(...)` call in `init()`.

#### 2. Increment site

Locate where `ArtifactBytesSentTotal.Add(n)` is called (the artifact receive handler that writes the zip into `agentfm_artifacts/<taskID>.zip`). The increment of `ArtifactsBuiltTotal.Inc()` belongs **after** the file has been fully written and renamed into place — partial writes do not count as assets built.

#### 3. Chat handler task counting

In `internal/boss/openai_chat.go`:

`streamChatCompletion` (around lines 102-162): wrap the body so that on exit (success or failure) it records:
- `metrics.TasksTotal.WithLabelValues(status).Inc()` where status = `metrics.StatusOK` if `ts.success` (the existing success-tracking field) else `metrics.StatusError`.
- `metrics.TaskDurationSeconds.Observe(elapsed)` where elapsed = `time.Since(start)` measured from before `b.openTaskStream(...)`.

`handleChatCompletions` non-streaming path (around lines 56-99): same pair of metric calls after `drainTaskStream` resolves (or fails). Treat the timeout branch as `status = metrics.StatusError`.

Rationale: chat completions are AgentFM tasks delivered through a different wire format. The metric is task-shaped, not transport-shaped — chat should count.

### Renderer side

#### 4. New selectors in `Dashboard.tsx`

After the existing `useMetricsStore` reads, add two memoized selectors:

```ts
const assetsBuiltBuf = bossSeries.get(seriesKey('agentfm_artifacts_built_total', {}))
const assetsBuiltCount = latestValue(assetsBuiltBuf ?? emptyBuf()) ?? 0

const successRateSeries = useMemo(
  () => computeSuccessRateSeries(bossSeries),
  [bossSeries],
)
const successRateNow = successRateSeries.at(-1) ?? 1
```

#### 5. New helper `computeSuccessRateSeries` in `lib/metricsDerive.ts`

```ts
/**
 * Computes a series of point-in-time success rates from the four
 * agentfm_tasks_total{status} ring buffers. At each timestamp t,
 *   rate(t) = ok(t) / (ok + error + rejected + timeout)(t)
 * Returns an array aligned to the OK buffer's timestamps. Empty array
 * when no OK series is available. Returns 1.0 when total denominator
 * is 0 (no traffic yet — treat as "perfect").
 */
export function computeSuccessRateSeries(
  buffers: Map<string, RingBuffer>,
): number[] { ... }
```

Aligns by index assuming all four series share the same poll cadence (they do — they come from the same scrape). If a non-OK series is missing, treat its contribution as 0.

#### 6. New tile renders

Replace the existing `<div className="grid grid-cols-3 gap-3.5 mb-4">` block that holds "Output speed / Sign-in attempts / Errors by channel" with a 4-tile row: those three + a new "Assets built" tile rendering `assetsBuiltCount` + a SparkLine over the assets ring buffer.

After that row, add a new full-width card:

```tsx
<NeonCard className="p-5 mb-4">
  <div className="text-[10px] font-mono ...">Success rate · last 2 min</div>
  <div className="text-[40px] font-mono font-bold leading-none mb-3"
       style={{ color: successRateColor(successRateNow) }}>
    {(successRateNow * 100).toFixed(1)}%
  </div>
  <SparkLine
    values={successRateSeries.map((r) => r * 100)}
    color={successRateColor(successRateNow)}
    min={0}
    max={100}
  />
</NeonCard>
```

`successRateColor`: lime `#84cc16` for ≥ 0.95, amber `#fbbf24` for 0.80–0.95, rose `#f43f5e` below 0.80.

#### 7. Animated number tickers

Create `src/components/dashboard/AnimatedNumber.tsx`:

```ts
import { motion, useMotionValue, useTransform, animate } from 'framer-motion'

export function AnimatedNumber({
  value,
  format = (n: number) => Math.round(n).toString(),
}: {
  value: number
  format?: (n: number) => string
}) {
  // ... tween from previous to new value over 500ms with cubic-out easing.
  // If |new - old| > old * 10, skip the tween (handles boss restart resets).
}
```

Use it on these four locations in `Dashboard.tsx`:
- "Tasks · last 5 min" big number
- "Agents online" tile value
- "Assets built" tile value
- "Success rate" % big number

#### 8. SparkLine pulse-dot tip

In `src/components/charts/SparkLine.tsx`, after the existing `<polyline>`, render an SVG `<circle>` at the last point's `(x, y)` with class `animate-pulse-cyan` (already defined in tailwind via the existing `pulse-cyan` keyframe). Skip when `values.length < 2` to avoid a stray dot on empty buffers.

Color of the dot matches the SparkLine's `color` prop.

#### 9. "Updated Ns ago" header

In `Dashboard.tsx`, replace the existing stale-only banner:

```tsx
{stale && <div className="text-[11px] font-mono text-warn">stale {Math.round(staleAgeMs / 1000)}s</div>}
```

with:

```tsx
<div className={`text-[11px] font-mono ${stale ? 'text-warn' : 'text-text-2'}`}>
  {lastTick === 0
    ? 'connecting…'
    : `updated ${(staleAgeMs / 1000).toFixed(1)}s ago`}
</div>
```

Update once per second via a `useState` + `setInterval` in `Dashboard.tsx` so the digit ticks visibly even when nothing else changes. Stop on unmount.

## Data flow

Unchanged from today:

1. `useMetricsPoll` fires every 2 s while the document is visible.
2. `fetch(/metrics)` returns Prometheus text exposition.
3. `parseMetrics(text)` returns an array of `{ name, labels, value }` samples.
4. `metricsStore.pushBoss(now, samples)` upserts each sample into its ring buffer keyed by `seriesKey(name, labels)`.
5. Dashboard route's selectors (memoized) recompute on store change; React re-renders.
6. `AnimatedNumber` tweens its DOM text from previous render value to new render value.
7. `SparkLine` re-draws the polyline + repositions the pulse-dot circle.

The new artifacts-built series joins the same pipeline.

## Error handling

- `useMetricsPoll` already absorbs HTTP errors with 3-strike → 10 s backoff. No change.
- Missing series: every selector defaults to `emptyBuf()` then `?? 0`. New selectors follow the same idiom.
- Boss restart resets all counters to 0. `AnimatedNumber` detects the drop by comparing `|new - old| > old * 10` and skips the tween in that case, preventing an unreadable count-down animation from `previous_value → 0`.
- `computeSuccessRateSeries` returns `[]` when no OK buffer exists and `1.0` when the denominator is 0 (no traffic = "perfect", not "broken").

## Testing

### Boss

- **New test** `TestChatCompletion_IncrementsTasksTotal` in `internal/boss/openai_test.go`:
  - Stand up a boss with a stub worker stream that immediately closes with success.
  - Reset `metrics.TasksTotal` before the call.
  - POST `/v1/chat/completions` (non-streaming) with `model = <worker peer id>` + one message.
  - Assert `metrics.TasksTotal.WithLabelValues("ok")` count incremented by exactly 1.
  - Assert `metrics.TaskDurationSeconds` sample count incremented by 1.
  - Symmetric test for the streaming path.
  - Symmetric test for the failure path → status="error".
- **New test** `TestArtifactsBuilt_IncrementsOnSuccess` in `internal/boss/artifacts_test.go` (or wherever the artifact-receive handler is tested):
  - Send a valid zip over the artifact protocol.
  - Assert `metrics.ArtifactsBuiltTotal` count incremented by 1.
  - Send a truncated stream → no increment.

### Renderer

- **Unit** `src/lib/__tests__/metricsDerive.test.ts`:
  - `computeSuccessRateSeries([])` returns `[]`.
  - All-OK series → all 1.0.
  - All-error series → all 0.0.
  - Mixed series at one tick → expected ratio.
  - Empty denominator at one tick → 1.0 (no division by zero).
- **Component** `src/components/dashboard/__tests__/AnimatedNumber.test.tsx`:
  - Mounts with initial value; updates value; assert intermediate frame text differs from final value (proves tween ran).
  - Updates value by > 10×; assert no intermediate frame (proves skip kicked in).
- **E2E** extend `tests/e2e/dashboard.spec.ts`:
  - Assert "Assets built" tile is visible.
  - Assert "Success rate" card is visible.
  - Assert the "updated Ns ago" indicator is visible (any digit + "s ago").
  - Do NOT assert numeric values — the test boss has no traffic.

## YAGNI

- Per-worker breakdown: deferred. Would need `worker` label on `TasksTotal` (cardinality + boss-side scope). Radar already shows per-worker activity.
- Latency p-line charts: deferred. Existing `p95` tile is sufficient until users ask for more.
- SSE / push: deferred. 2 s poll + animation polish is the right cost for this iteration.
- Dashboard preferences (animation toggle, window selector): deferred. Defaults look good.

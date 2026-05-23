# Real-time Metrics Dashboard — Desktop App

**Status:** Draft · 2026-05-23
**Scope:** `agentfm-desktop` (renderer only). Zero changes to `agentfm-core/agentfm-go`.

> **Path convention.** All `src/…` paths in this document are relative to the `agentfm-desktop/` repository root (the Electron app). All `internal/…` paths are relative to `agentfm-core/agentfm-go/`. The spec lives in `agentfm-core/` because AgentFM is a single product spread across two sibling repos.

## Summary

Add a real-time observability surface to the Electron desktop app, sourced from the boss API's existing `/metrics` endpoint and the existing `/api/workers` telemetry poll. Two surfaces:

1. A new top-level **`/dashboard`** route showing boss-level signals (task throughput, durations, stream errors, auth attempts, runtime health).
2. A **`<TelemetryStrip>`** band on each **`/peer/:peerId`** view showing live CPU/GPU/RAM-free/queue sparklines for that worker.

The boss API already exposes `/metrics` on port 8080 unauthenticated and CORS-free (see `internal/boss/api.go:157`). No backend work is required.

## Goals

- A renderer-only feature; no Go or Electron-main changes.
- Live charts that update every 2 seconds and hold the last 5 minutes of history client-side.
- Negligible bundle and memory footprint (single small chart library, ring buffers in renderer memory).
- Match existing UI rhythm — neon palette, mono labels, NeonCard surfaces — so the dashboard reads as a first-class route alongside Radar / Status.

## Non-goals

- No on-disk persistence of metric history. Restarting the app starts the buffer fresh.
- No remote scraping. Boss and the desktop run on the same host; we do not attempt to scrape worker `/metrics` endpoints over libp2p.
- No new boss endpoint or new HTTP route. We consume what `/metrics` already exposes.
- No alerting, no thresholds, no notifications. This is a glance-tool, not an oncall pager.
- No multi-window support. The renderer-driven design assumes a single open window today.

## Data sources

| Source | Endpoint | Cadence | Used by |
|---|---|---|---|
| Boss Prometheus metrics | `GET http://127.0.0.1:8080/metrics` | 2 s poll while `/dashboard` visible | `Dashboard.tsx` tiles |
| Worker telemetry snapshots | `GET /api/workers` (polled by existing `useWorkers` React Query hook in `src/lib/query.ts`) | 2 s (existing) | `TelemetryStrip.tsx` on `PeerView` |

### Metrics consumed (from `internal/metrics/metrics.go`)

| Family | Type | Labels | Used for |
|---|---|---|---|
| `agentfm_tasks_total` | counter | `status ∈ {ok,error,rejected,timeout}` | Hero tile: tasks-per-min, status breakdown |
| `agentfm_task_duration_seconds` | histogram | (buckets 1s–30min) | P95 duration tile (client-side interp) |
| `agentfm_workers_online` | gauge | — | Workers-online tile |
| `agentfm_artifact_bytes_sent_total` | counter | — | Artifact bytes/sec tile (derivative) |
| `agentfm_stream_errors_total` | counter | `protocol`, `reason` | Stream-errors tile + protocol-breakdown bar |
| `agentfm_auth_attempts_total` | counter | `outcome` | Auth-attempts bar |
| `process_cpu_seconds_total` | counter | — | Runtime CPU% (derivative × 100) |
| `process_resident_memory_bytes` | gauge | — | Runtime RSS |
| `go_goroutines` | gauge | — | Runtime goroutines |
| `go_gc_duration_seconds` | summary | — | Runtime GC p95 |

`agentfm_dht_queries_total` (relay-only) is intentionally **not** consumed — the desktop talks to a boss, not a relay.

### Worker profile fields consumed (from existing `WorkerProfile` type in `src/types/api.ts`)

| Field | Used as |
|---|---|
| `cpu_usage_pct` | CPU sparkline (0–100 %) |
| `gpu_usage_pct` | GPU sparkline (0–100 %) |
| `ram_free_gb` | RAM-free sparkline (GB, shown as "RAM FREE") |
| `current_tasks` | Queue sparkline (count) |
| `peer_id`, `last_seen`, `online` | Buffer key / offline detection |

## Architecture

Pure renderer-side. The high-level diagram:

```
boss API (existing, :8080)                            renderer (Electron + React)
├─ GET /metrics       ────────► useMetricsPoll (2s) ──► promParse ──┐
└─ GET /api/workers   ────────► useWorkerHistory     ───────────────┤
                                                                    ▼
                                              useMetricsStore (Zustand, ring buffers)
                                                                    │
                              ┌─────────────────────────────────────┴────────────┐
                              ▼                                                  ▼
                       Dashboard.tsx                                  TelemetryStrip.tsx
                       (UPlotChart + tiles)                           (SparkLine × 4)
```

### Component inventory

**New files (8):**

1. `src/lib/promParse.ts` — `parseMetrics(text: string): MetricSample[]`. Pure function. Recognises `# HELP`/`# TYPE` (skipped), counter/gauge lines, and histogram triples (`_bucket{le="..."} N`, `_sum`, `_count`). Defensive: each line is wrapped in try/catch; malformed or unknown-name lines are silently dropped.

2. `src/hooks/useMetricsPoll.ts` — owns the 2 s `setInterval` against `/metrics`. Pauses when `document.visibilityState !== 'visible'`. Backs off to 10 s after 3 consecutive errors; returns to 2 s on first success.

3. `src/lib/metricsStore.ts` — new Zustand store. Two ring buffers:
   - `bossSeries: Map<seriesKey, RingBuffer>` keyed by `name+labels` (e.g. `agentfm_tasks_total{status=ok}`). Each `RingBuffer = { ts: Float64Array(150), v: Float64Array(150), head: number }` — 150 = 5 min ÷ 2 s.
   - `peerSeries: Map<peerId, Map<metric, RingBuffer>>` populated by `useWorkerHistory`.
   - `pushBoss(ts, samples)` advances every known series. Series not present in this tick get filled with their previous value carried forward (so charts never gap on a single missing data point).

4. `src/components/charts/UPlotChart.tsx` — thin React wrapper around `uplot`. Props: `series: RingBuffer[]`, `labels: string[]`, `colors: string[]`, `height`. One `uPlot` instance per chart, kept in a `ref`; `setData()` called via a `useEffect` that subscribes to the relevant store slice. Cleanup calls `chart.destroy()` to survive React 18 StrictMode double-invoke.

5. `src/components/charts/SparkLine.tsx` — minimal canvas-based single-series sparkline (~30 lines). Used for the 4 PeerView cells where uPlot would be overkill.

6. `src/routes/Dashboard.tsx` — the Hero+tiles layout:
   - Hero card: `agentfm_tasks_total` sum across statuses (5-min sparkline) + status breakdown (ok/error/rejected/timeout).
   - Row 2 (3 tiles): P95 task duration (histogram interp), Workers online (`agentfm_workers_online`), Stream errors total (5-min delta).
   - Row 3 (3 tiles): Errors by protocol (small stacked bar), Auth attempts (small bar), Artifact bytes/sec (rate, sparkline).
   - Row 4 (4 small tiles): CPU%, RSS, Goroutines, GC pause p95.

7. `src/components/peer/TelemetryStrip.tsx` — 4-cell strip (CPU/GPU/RAM/Queue) using `SparkLine`. Shows the existing "Waiting for telemetry beacon…" placeholder when buffer empty. Shows "(offline — last seen Nm ago)" when `Date.now() - lastTickTs > 30_000`. Re-renders subscribed to one store slice (`peerSeries.get(peerId)`).

8. `src/hooks/useWorkerHistory.ts` — runs continuously app-wide (mounted in `App.tsx`). Subscribes to the existing React Query `/api/workers` cache; on every successful refetch, iterates the profile list and pushes each peer's CPU/GPU/RAM-free/queue into `peerSeries`. Decoupling history capture from PeerView mount means navigating to a peer page shows charts immediately.

**Modified files (3):**

9. `src/components/TabStrip.tsx` — add a `Dashboard` entry to the `tabs` array between Radar and Activity. Match existing label rhythm (no icon column — TabStrip is text-only).

10. `src/App.tsx` — register `<Route path="/dashboard" element={<Dashboard />} />`. Call `useWorkerHistory()` directly inside `App` so it runs for the lifetime of the renderer (no wrapper component needed).

11. `src/routes/PeerView.tsx` — insert `<TelemetryStrip peerId={summary.peer_id} />` between the existing `<SummaryCard>` (line 95) and `<Tabs>` (line 98).

**New dependency:**

- `uplot` (~45 KB minified). No `react-uplot`; we wrap directly.

## Data flow

### Boss /metrics pipeline (every 2 s, while `/dashboard` is visible)

```
fetch('/metrics')
  └─→ text/plain
        │
        ▼
parseMetrics(text)
  └─→ [{name, labels, value, type}, ...]
        │
        ▼
metricsStore.pushBoss(Date.now(), samples)
  └─→ for each series: ring[head]={ts,v}; head=(head+1)%150
       missing series: carry-forward previous value
        │
        ▼
Zustand subscribers re-render
  └─→ <UPlotChart> chart.setData(buffer)
       <Tile> reads buffer.latestValue()
```

### Per-worker telemetry pipeline (always running, app-wide)

```
React Query /api/workers (existing 2s poll)
  └─→ WorkerProfile[]
        │
        ▼
useWorkerHistory pushes every snapshot
  └─→ for each profile: peerSeries[peerId][metric].push(now, value)
        │
        ▼
<TelemetryStrip peerId={x}> subscribed to peerSeries[x]
```

### Derived values (computed at render-time)

| Derived | Source | Computation |
|---|---|---|
| Tasks-per-minute | `agentfm_tasks_total` counter buffer | `(latest − value_at(now − 60s)) / 60` |
| Task duration p95 | `agentfm_task_duration_seconds_bucket` | Linear interp on cumulative-count vector |
| Artifact bytes/sec | `agentfm_artifact_bytes_sent_total` | First derivative over buffer Δt |
| CPU% | `process_cpu_seconds_total` | First derivative × 100 |

All four wrapped in `useMemo` keyed to the relevant buffer's `head`.

### Pause / resume

- `document.visibilityState === 'hidden'` → polling pauses, no network requests. Store retained.
- Visible again → next tick is a wider Δ; derivatives absorb naturally.
- User navigates away from `/dashboard` → `/metrics` polling stops. Per-worker history keeps going (it piggybacks on `/api/workers` which always polls).

## Error handling

| Failure | Behaviour |
|---|---|
| Boss unreachable (network error / non-2xx) | Existing `useBackend` `BackendDownOverlay` covers the whole route. `useMetricsPoll` independently backs off to 10 s after 3 errors; recovers on first success. |
| Single failed poll | Carry-forward keeps charts gap-free. A "stale Ns" badge appears in the dashboard header once last-success > 10 s. |
| Malformed `/metrics` line | `parseMetrics` skips the line silently. In `import.meta.env.DEV`, log a single `console.warn` on first unknown metric name encountered. |
| Unknown metric name (e.g. new metric added Go-side) | Ignored. Adding a chart for it later requires only a config addition; no parser change. |
| Peer disappears from `/api/workers` > 30 s | `TelemetryStrip` shows "(offline — last seen Nm ago)" and stops appending. Buffer retained 5 min so quick disconnect/reconnect preserves history. |
| Peer never published telemetry | Strip shows "Waiting for telemetry beacon…" placeholder (reuses radar-skeleton copy). |
| Equivocator peer | Strip still renders (telemetry is data, not trust). Existing red equivocator banner above provides the warning context. |
| uPlot instance leak under React 18 StrictMode | `useEffect` cleanup calls `chart.destroy()`. |
| `/metrics` returns HTML (proxy mishap, wrong port) | Every line fails to parse; tick produces zero samples; stale badge surfaces. |

**Explicitly out of scope:** no toasts, no retry-with-backoff for individual fetches (the 2 s loop is its own backoff), no fallback parser.

## Testing

Match the existing layout in `tests/{unit,e2e}/`. Test framework: Vitest (unit) + Playwright (e2e), already configured in `package.json`.

### Unit (Vitest)

**`promParse.ts`** — the only piece with non-trivial parsing:
- Real `/metrics` output captured from a running boss, saved as `tests/unit/fixtures/metrics-sample.txt`. Round-trip asserts the expected sample count and types.
- Histogram with `_bucket`/`_sum`/`_count` lines collapses into one histogram entity.
- `# HELP` and `# TYPE` lines skipped without error.
- Malformed line mid-file → that line dropped, rest parses.
- Empty input → returns `[]`.

**`metricsStore.ts` ring buffer:**
- 150 pushes fill the buffer; push #151 overwrites slot 0; `latest()` returns push #151.
- Carry-forward: pushing a tick with a subset of series leaves untouched series with their previous value at the new timestamp.
- Per-peer isolation: pushes to peer A do not appear in peer B's series.

**Derived computations** (`computeRate`, `computeP95FromBuckets`, `computeTasksPerMinute`) — table-driven tests covering empty buffer, single sample, steady rate.

### Component smoke (Vitest + Testing Library)

- `<Dashboard>` renders without crashing on empty store (shows skeletons).
- `<Dashboard>` renders expected tiles when store is seeded with one tick.
- `<TelemetryStrip>` shows placeholder when no buffer; strip when buffer has data; offline copy when `lastTickAgo > 30s`.

These are smoke tests, not full DOM snapshots.

### E2E (Playwright)

One happy-path test: `tests/e2e/dashboard.spec.ts` intercepts `/metrics` via Playwright route-mocking, serves the saved fixture, navigates to `/dashboard`, asserts the hero tile shows the expected task count. Waits one poll cycle (2 s); asserts the chart re-rendered with a second sample. Mocking is preferable to spinning up a real boss for a renderer-only feature.

### Manual verification (PR checklist)

- Real worker + boss running locally; open `/dashboard`; watch 30 s — sparklines move, no console errors.
- Stop boss → "stale Ns" badge appears within ~10 s; backend-down overlay activates.
- Restart boss → polling resumes; badge clears.
- Open PeerView for an online worker → telemetry strip populates within 2 s.
- Background the window for a minute → no `/metrics` requests in DevTools network tab.

### Explicitly out of scope

- No screenshot / visual-regression tests — uPlot canvas output is noisy under headless rendering.
- No load tests.
- No tests for uPlot itself — trust the library.

## Open questions

None at design close — every fork has been resolved during brainstorming. Implementation plan to follow in a separate document.

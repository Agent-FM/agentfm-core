import { useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { useMetricsPoll } from '../hooks/useMetricsPoll'
import { useMetricsStore, seriesKey } from '../lib/metricsStore'
import {
  computeRate,
  computeTasksPerMinute,
  computeP95FromBuckets,
  computeSuccessRateSeries,
} from '../lib/metricsDerive'
import { createRingBuffer, latestValue, ringToArrays } from '../types/metrics'
import { SparkLine } from '../components/charts/SparkLine'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { Card } from '../components/primitives/Card'
import { AnimatedNumber } from '../components/dashboard/AnimatedNumber'
import { COLORS } from '../lib/colors'

const STATUSES = ['ok', 'error', 'rejected', 'timeout'] as const
const STATUS_COLOR: Record<(typeof STATUSES)[number], string> = {
  ok: COLORS.ok,
  error: COLORS.bad,
  rejected: COLORS.accent,
  timeout: COLORS.warn,
}

const STALE_MS = 10_000

export default function Dashboard() {
  useMetricsPoll()
  const bossSeries = useMetricsStore((s) => s.bossSeries)
  const lastTick = useMetricsStore((s) => s.lastBossTick)

  const staleAgeMs = lastTick === 0 ? 0 : Date.now() - lastTick
  const stale = lastTick !== 0 && staleAgeMs > STALE_MS

  const [, forceTick] = useState(0)
  useEffect(() => {
    const id = setInterval(() => forceTick((n) => n + 1), 1000)
    return () => clearInterval(id)
  }, [])

  const taskCounts = useMemo(() => {
    return STATUSES.map((status) => {
      const buf = bossSeries.get(seriesKey('agentfm_tasks_total', { status }))
      return { status, value: buf ? latestValue(buf) ?? 0 : 0, buf }
    })
  }, [bossSeries])

  const totalTasks = taskCounts.reduce((a, b) => a + b.value, 0)

  const tasksPerMin = useMemo(() => {
    const okBuf = bossSeries.get(seriesKey('agentfm_tasks_total', { status: 'ok' }))
    return okBuf ? computeTasksPerMinute(okBuf) : 0
  }, [bossSeries])

  const p95Duration = useMemo(() => {
    const buckets: { le: number; count: number }[] = []
    for (const [k, buf] of bossSeries) {
      if (!k.startsWith('agentfm_task_duration_seconds_bucket{')) continue
      const m = k.match(/le=([^,}]+)/)
      if (!m) continue
      const leStr = m[1]
      const le = leStr === '+Inf' ? Infinity : Number(leStr)
      buckets.push({ le, count: latestValue(buf) ?? 0 })
    }
    return computeP95FromBuckets(buckets)
  }, [bossSeries])

  const workersOnline =
    latestValue(bossSeries.get(seriesKey('agentfm_workers_online', {})) ?? emptyBuf()) ?? 0

  const streamErrorsTotal = useMemo(() => {
    let n = 0
    for (const [k, buf] of bossSeries) {
      if (!k.startsWith('agentfm_stream_errors_total{')) continue
      n += latestValue(buf) ?? 0
    }
    return n
  }, [bossSeries])

  const cpuPct = useMemo(() => {
    const buf = bossSeries.get(seriesKey('process_cpu_seconds_total', {}))
    return buf ? computeRate(buf) * 100 : 0
  }, [bossSeries])

  const rssBytes =
    latestValue(bossSeries.get(seriesKey('process_resident_memory_bytes', {})) ?? emptyBuf()) ?? 0
  const goroutines =
    latestValue(bossSeries.get(seriesKey('go_goroutines', {})) ?? emptyBuf()) ?? 0

  const gcPauseP95 = useMemo(() => {
    for (const [k, buf] of bossSeries) {
      if (!k.startsWith('go_gc_duration_seconds{')) continue
      if (!k.includes('quantile=0.75')) continue
      return latestValue(buf) ?? 0
    }
    return 0
  }, [bossSeries])

  const errorsByProtocol = useMemo(() => {
    const out = new Map<string, number>()
    for (const [k, buf] of bossSeries) {
      if (!k.startsWith('agentfm_stream_errors_total{')) continue
      const m = k.match(/protocol=([^,}]+)/)
      if (!m) continue
      const proto = m[1]
      out.set(proto, (out.get(proto) ?? 0) + (latestValue(buf) ?? 0))
    }
    return Array.from(out.entries())
  }, [bossSeries])

  const assetsBuiltBuf = bossSeries.get(seriesKey('agentfm_artifacts_built_total', {}))
  const assetsBuiltCount = latestValue(assetsBuiltBuf ?? emptyBuf()) ?? 0
  const assetsBuiltValues = assetsBuiltBuf ? ringToArrays(assetsBuiltBuf).v : []

  const successRateSeries = useMemo(
    () => computeSuccessRateSeries(bossSeries),
    [bossSeries],
  )
  const successRateNow =
    successRateSeries.length > 0
      ? (successRateSeries[successRateSeries.length - 1] ?? 1)
      : 1
  const successRateColor =
    successRateNow >= 0.95
      ? COLORS.ok
      : successRateNow >= 0.8
        ? COLORS.warn
        : COLORS.bad

  const okBuf = bossSeries.get(seriesKey('agentfm_tasks_total', { status: 'ok' }))
  const heroValues = okBuf ? ringToArrays(okBuf).v : []

  return (
    <div className="p-4 max-w-6xl">
      <div className="flex justify-between items-end mb-4">
        <div>
          <HeroTitle accent="mesh">Live</HeroTitle>
          <div className="flex items-center gap-4 text-2xs text-text-2 mt-1">
            <span>
              <span className="text-accent font-mono tabular-nums">{Math.round(totalTasks)}</span> tasks since start
            </span>
            <span>
              <span className="text-accent font-mono tabular-nums">{tasksPerMin.toFixed(1)}</span> per minute
            </span>
          </div>
        </div>
        <div className={`text-2xs font-mono tabular-nums ${stale ? 'text-warn' : 'text-text-2'}`}>
          {lastTick === 0 ? 'connecting…' : `updated ${(staleAgeMs / 1000).toFixed(1)}s ago`}
        </div>
      </div>

      <div className="grid grid-cols-12 gap-3 items-stretch">
        {/* left column: throughput */}
        <Card className="col-span-8 h-full">
          <div className="text-2xs font-medium text-text-2 mb-1.5">
            Total tasks
          </div>
          <div className="text-3xl font-mono font-semibold tabular-nums text-accent leading-none mb-2">
            <AnimatedNumber value={totalTasks} />
          </div>
          <div className="flex gap-4 text-xs font-mono mb-2">
            {taskCounts.map((t) => (
              <span key={t.status} style={{ color: STATUS_COLOR[t.status] }}>
                <span className="tabular-nums">{t.value}</span>{' '}
                {t.status === 'ok' ? 'completed' : t.status}
              </span>
            ))}
          </div>
          <SparkLine values={heroValues} height={32} color={COLORS.accent} />
        </Card>

        {/* right column: success rate */}
        <Card className="col-span-4 h-full">
          <div className="text-2xs font-medium text-text-2 mb-1.5">
            Success rate
          </div>
          <div
            className="text-3xl font-mono font-semibold tabular-nums leading-none mb-2"
            style={{ color: successRateColor }}
          >
            <AnimatedNumber
              value={successRateNow}
              format={(n) => `${(n * 100).toFixed(1)}%`}
            />
          </div>
          <SparkLine
            values={successRateSeries.map((r) => r * 100)}
            height={32}
            color={successRateColor}
          />
        </Card>

        {/* KPI row */}
        <div className="col-span-4">
          <Tile
            label="Typical task time"
            value={<span className="tabular-nums">{`${p95Duration.toFixed(1)}s`}</span>}
            color={COLORS.accent}
          />
        </div>
        <div className="col-span-4">
          <Tile label="Agents online" value={<AnimatedNumber value={workersOnline} />} color={COLORS.accent} />
        </div>
        <div className="col-span-4">
          <Tile label="Connection errors" value={<span className="tabular-nums">{Math.round(streamErrorsTotal)}</span>} color={COLORS.bad} />
        </div>

        {/* assets + errors */}
        <Card className="col-span-6 h-full">
          <div className="text-2xs font-medium text-text-2 mb-1.5">
            Assets built
          </div>
          <div className="text-2xl font-mono font-semibold tabular-nums text-accent leading-none mb-1.5">
            <AnimatedNumber value={assetsBuiltCount} />
          </div>
          <SparkLine values={assetsBuiltValues} height={24} color={COLORS.accent} />
        </Card>
        <Card className="col-span-6 h-full">
          <div className="text-2xs font-medium text-text-2 mb-1.5">
            Errors by channel
          </div>
          {errorsByProtocol.length === 0 ? (
            <div className="font-mono text-text-2 text-xs">No errors yet</div>
          ) : (
            <div>
              {errorsByProtocol.map(([proto, n]) => (
                <div key={proto} className="flex justify-between items-center h-6 font-mono text-xs border-b border-border-0 last:border-b-0 hover:bg-white/[0.04]">
                  <span className="text-text-1">{proto}</span>
                  <span className="tabular-nums text-right" style={{ color: COLORS.bad }}>{Math.round(n)}</span>
                </div>
              ))}
            </div>
          )}
        </Card>

        {/* boss process health */}
        <div className="col-span-12 mt-1 text-2xs font-medium text-text-2">
          Boss process health
        </div>
        <div className="col-span-3">
          <Tile label="CPU load" value={<span className="tabular-nums">{`${cpuPct.toFixed(1)}%`}</span>} color={COLORS.ok} />
        </div>
        <div className="col-span-3">
          <Tile
            label="Memory used"
            value={<span className="tabular-nums">{`${(rssBytes / 1024 / 1024).toFixed(0)} MB`}</span>}
            color={COLORS.ok}
          />
        </div>
        <div className="col-span-3">
          <Tile label="Background tasks" value={<span className="tabular-nums">{Math.round(goroutines)}</span>} color={COLORS.ok} />
        </div>
        <div className="col-span-3">
          <Tile
            label="Pause time"
            value={<span className="tabular-nums">{`${(gcPauseP95 * 1000).toFixed(1)} ms`}</span>}
            hint="cleanup pause on the boss"
            color={COLORS.ok}
          />
        </div>
      </div>
    </div>
  )
}

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
    <Card className="h-full flex flex-col">
      <div className="text-2xs font-medium text-text-2 mb-1.5">
        {label}
      </div>
      <div className="text-2xl font-mono font-semibold tabular-nums leading-none" style={{ color }}>
        {value}
      </div>
      {hint && (
        <div className="text-2xs text-text-2 mt-auto pt-1.5">{hint}</div>
      )}
    </Card>
  )
}

function emptyBuf() {
  return createRingBuffer()
}

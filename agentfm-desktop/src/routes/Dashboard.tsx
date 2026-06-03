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
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { NeonCard } from '../components/primitives/NeonCard'
import { AnimatedNumber } from '../components/dashboard/AnimatedNumber'

const STATUSES = ['ok', 'error', 'rejected', 'timeout'] as const
const STATUS_COLOR: Record<(typeof STATUSES)[number], string> = {
  ok: '#84cc16',
  error: '#f43f5e',
  rejected: '#a855f7',
  timeout: '#fbbf24',
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

  const artifactBytesPerSec = useMemo(() => {
    const buf = bossSeries.get(seriesKey('agentfm_artifact_bytes_sent_total', {}))
    return buf ? computeRate(buf) : 0
  }, [bossSeries])

  const cpuPct = useMemo(() => {
    const buf = bossSeries.get(seriesKey('process_cpu_seconds_total', {}))
    return buf ? computeRate(buf) * 100 : 0
  }, [bossSeries])

  const rssBytes =
    latestValue(bossSeries.get(seriesKey('process_resident_memory_bytes', {})) ?? emptyBuf()) ?? 0
  const goroutines =
    latestValue(bossSeries.get(seriesKey('go_goroutines', {})) ?? emptyBuf()) ?? 0

  const authAttemptsTotal = useMemo(() => {
    let n = 0
    for (const [k, buf] of bossSeries) {
      if (!k.startsWith('agentfm_auth_attempts_total{')) continue
      n += latestValue(buf) ?? 0
    }
    return n
  }, [bossSeries])

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
      ? '#84cc16'
      : successRateNow >= 0.8
        ? '#fbbf24'
        : '#f43f5e'

  const okBuf = bossSeries.get(seriesKey('agentfm_tasks_total', { status: 'ok' }))
  const heroValues = okBuf ? ringToArrays(okBuf).v : []

  return (
    <div className="p-7 max-w-6xl">
      <div className="flex justify-between items-center">
        <SectionLabel>DASHBOARD</SectionLabel>
        <div className={`text-[11px] font-mono ${stale ? 'text-warn' : 'text-text-2'}`}>
          {lastTick === 0
            ? 'connecting…'
            : `updated ${(staleAgeMs / 1000).toFixed(1)}s ago`}
        </div>
      </div>
      <HeroTitle accent="mesh">Live</HeroTitle>
      <p className="text-[16px] text-text-1 mt-2 mb-7">
        TASKS · {Math.round(totalTasks)} total · ▲ {tasksPerMin.toFixed(1)}/min
      </p>

      <NeonCard className="p-5 mb-4">
        <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
          Tasks · last 5 min
        </div>
        <div className="text-[40px] font-mono font-bold text-accent leading-none mb-3">
          <AnimatedNumber value={totalTasks} />
        </div>
        <div className="flex gap-4 text-xs font-mono mb-3">
          {taskCounts.map((t) => (
            <span key={t.status} style={{ color: STATUS_COLOR[t.status] }}>
              {t.value} {t.status === 'ok' ? 'completed' : t.status}
            </span>
          ))}
        </div>
        <SparkLine values={heroValues} width={800} height={40} color="#22d3ee" />
      </NeonCard>

      <div className="grid grid-cols-3 gap-3.5 mb-4">
        <Tile
          label="Typical task time"
          value={`${p95Duration.toFixed(1)}s`}
          hint="9 in 10 tasks finish faster"
          color="#22d3ee"
        />
        <Tile label="Agents online" value={<AnimatedNumber value={workersOnline} />} color="#a855f7" />
        <Tile label="Connection errors" value={`${Math.round(streamErrorsTotal)}`} color="#f43f5e" />
      </div>

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
          <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
            Errors by channel
          </div>
          {errorsByProtocol.length === 0 ? (
            <div className="font-mono text-text-2 text-xs">(none)</div>
          ) : (
            <div className="space-y-1">
              {errorsByProtocol.map(([proto, n]) => (
                <div key={proto} className="flex justify-between font-mono text-xs">
                  <span className="text-text-1">{proto}</span>
                  <span style={{ color: '#f43f5e' }}>{Math.round(n)}</span>
                </div>
              ))}
            </div>
          )}
        </NeonCard>
      </div>

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

      <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
        Boss process health
      </div>
      <div className="grid grid-cols-4 gap-3.5">
        <Tile label="CPU load" value={`${cpuPct.toFixed(1)}%`} color="#84cc16" />
        <Tile
          label="Memory used"
          value={`${(rssBytes / 1024 / 1024).toFixed(0)} MB`}
          color="#84cc16"
        />
        <Tile label="Background tasks" value={`${Math.round(goroutines)}`} color="#84cc16" />
        <Tile
          label="Pause time"
          value={`${(gcPauseP95 * 1000).toFixed(1)} ms`}
          hint="cleanup pause on the boss"
          color="#84cc16"
        />
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

function emptyBuf() {
  return createRingBuffer()
}

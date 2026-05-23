import { useMemo } from 'react'
import { useMetricsPoll } from '../hooks/useMetricsPoll'
import { useMetricsStore, seriesKey } from '../lib/metricsStore'
import {
  computeRate,
  computeTasksPerMinute,
  computeP95FromBuckets,
} from '../lib/metricsDerive'
import { createRingBuffer, latestValue, ringToArrays } from '../types/metrics'
import { SparkLine } from '../components/charts/SparkLine'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { NeonCard } from '../components/primitives/NeonCard'

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

  const okBuf = bossSeries.get(seriesKey('agentfm_tasks_total', { status: 'ok' }))
  const heroValues = okBuf ? ringToArrays(okBuf).v : []

  return (
    <div className="p-7 max-w-6xl">
      <div className="flex justify-between items-center">
        <SectionLabel>DASHBOARD</SectionLabel>
        {stale && (
          <div className="text-[11px] font-mono text-warn">
            stale {Math.round(staleAgeMs / 1000)}s
          </div>
        )}
      </div>
      <HeroTitle accent="mesh">Live</HeroTitle>
      <p className="text-[16px] text-text-1 mt-2 mb-7">
        TASKS · {Math.round(totalTasks)} total · ▲ {tasksPerMin.toFixed(1)}/min
      </p>

      <NeonCard className="p-5 mb-4">
        <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
          TASKS · LAST 5 MIN
        </div>
        <div className="text-[40px] font-mono font-bold text-accent leading-none mb-3">
          {Math.round(totalTasks)}
        </div>
        <div className="flex gap-4 text-xs font-mono mb-3">
          {taskCounts.map((t) => (
            <span key={t.status} style={{ color: STATUS_COLOR[t.status] }}>
              {t.value} {t.status}
            </span>
          ))}
        </div>
        <SparkLine values={heroValues} width={800} height={40} color="#22d3ee" />
      </NeonCard>

      <div className="grid grid-cols-3 gap-3.5 mb-4">
        <Tile label="P95 DURATION" value={`${p95Duration.toFixed(1)}s`} color="#22d3ee" />
        <Tile label="WORKERS ONLINE" value={`${Math.round(workersOnline)}`} color="#a855f7" />
        <Tile label="STREAM ERRORS" value={`${Math.round(streamErrorsTotal)}`} color="#f43f5e" />
      </div>

      <div className="grid grid-cols-3 gap-3.5 mb-4">
        <Tile
          label="ARTIFACT BYTES/SEC"
          value={`${(artifactBytesPerSec / 1024).toFixed(1)} KB/s`}
          color="#22d3ee"
        />
        <Tile label="AUTH ATTEMPTS" value={`${Math.round(authAttemptsTotal)}`} color="#a855f7" />
        <NeonCard className="p-4">
          <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
            ERRORS BY PROTOCOL
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

      <div className="grid grid-cols-4 gap-3.5">
        <Tile label="CPU%" value={`${cpuPct.toFixed(1)}%`} color="#84cc16" />
        <Tile
          label="RSS"
          value={`${(rssBytes / 1024 / 1024).toFixed(0)}M`}
          color="#84cc16"
        />
        <Tile label="GOROUTINES" value={`${Math.round(goroutines)}`} color="#84cc16" />
        <Tile
          label="GC PAUSE p75"
          value={`${(gcPauseP95 * 1000).toFixed(1)}ms`}
          color="#84cc16"
        />
      </div>
    </div>
  )
}

function Tile({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <NeonCard className="p-4">
      <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 mb-2">
        {label}
      </div>
      <div className="font-mono font-bold leading-none" style={{ fontSize: 28, color }}>
        {value}
      </div>
    </NeonCard>
  )
}

function emptyBuf() {
  return createRingBuffer()
}

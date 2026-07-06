import { useMetricsStore } from '../../lib/metricsStore'
import { SparkLine } from '../charts/SparkLine'
import { latestValue, ringToArrays } from '../../types/metrics'
import type { PeerMetric } from '../../lib/metricsStore'

const OFFLINE_AFTER_MS = 30_000

const CELLS: { metric: PeerMetric; label: string; color: string; fmt: (v: number) => string }[] = [
  { metric: 'cpu', label: 'CPU', color: '#F7931E', fmt: (v) => `${Math.round(v)}%` },
  { metric: 'gpu', label: 'GPU', color: '#FBBF6B', fmt: (v) => `${Math.round(v)}%` },
  { metric: 'ram', label: 'RAM FREE', color: '#34d399', fmt: (v) => `${v.toFixed(1)}G` },
  { metric: 'queue', label: 'QUEUE', color: '#f59e0b', fmt: (v) => `${Math.round(v)}` },
]

function formatAgo(ms: number): string {
  if (ms < 60_000) return `${Math.round(ms / 1000)}s`
  if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m`
  return `${Math.round(ms / 3_600_000)}h`
}

export interface TelemetryStripProps {
  peerId: string
}

export function TelemetryStrip({ peerId }: TelemetryStripProps) {
  const peerBufs = useMetricsStore((s) => s.peerSeries.get(peerId))
  const lastTick = useMetricsStore((s) => s.peerLastTick.get(peerId))

  if (!peerBufs || !lastTick) {
    return (
      <div className="bg-bg-2 border border-border-0 rounded-[14px] p-4 text-text-2 text-sm">
        Waiting for telemetry beacon…
      </div>
    )
  }

  const sinceLast = Date.now() - lastTick
  const offline = sinceLast > OFFLINE_AFTER_MS

  return (
    <div className="bg-bg-2 border border-border-0 rounded-[14px] p-3">
      <div className="flex items-center justify-between mb-2">
        <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 flex items-center gap-1.5 tabular-nums">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              offline ? 'bg-text-2' : 'bg-ok'
            }`}
          />
          {offline ? `OFFLINE — last seen ${formatAgo(sinceLast)} ago` : 'LIVE TELEMETRY · LAST 5 MIN'}
        </div>
      </div>
      <div className="grid grid-cols-4 gap-2">
        {CELLS.map((c) => {
          const buf = peerBufs.get(c.metric)!
          const v = latestValue(buf) ?? 0
          const { v: values } = ringToArrays(buf)
          return (
            <div key={c.metric} className="bg-bg-1 border border-border-0 rounded-md p-2">
              <div className="text-[9px] font-mono uppercase tracking-wider text-text-2 mb-1">
                {c.label}
              </div>
              <div
                className="font-mono font-bold leading-none mb-1 tabular-nums"
                style={{ fontSize: 16, color: c.color }}
              >
                {c.fmt(v)}
              </div>
              <SparkLine values={values} width={80} height={24} color={c.color} />
            </div>
          )
        })}
      </div>
    </div>
  )
}

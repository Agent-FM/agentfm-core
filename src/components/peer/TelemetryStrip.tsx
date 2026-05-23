import { useMetricsStore } from '../../lib/metricsStore'
import { SparkLine } from '../charts/SparkLine'
import { latestValue, ringToArrays } from '../../types/metrics'
import type { PeerMetric } from '../../lib/metricsStore'

const OFFLINE_AFTER_MS = 30_000

const CELLS: { metric: PeerMetric; label: string; color: string; fmt: (v: number) => string }[] = [
  { metric: 'cpu', label: 'CPU', color: '#22d3ee', fmt: (v) => `${Math.round(v)}%` },
  { metric: 'gpu', label: 'GPU', color: '#a855f7', fmt: (v) => `${Math.round(v)}%` },
  { metric: 'ram', label: 'RAM FREE', color: '#84cc16', fmt: (v) => `${v.toFixed(1)}G` },
  { metric: 'queue', label: 'QUEUE', color: '#f43f5e', fmt: (v) => `${Math.round(v)}` },
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
      <div className="border border-violet-500/20 bg-violet-500/5 rounded-lg p-4 text-text-2 text-sm">
        Waiting for telemetry beacon…
      </div>
    )
  }

  const sinceLast = Date.now() - lastTick
  const offline = sinceLast > OFFLINE_AFTER_MS

  return (
    <div className="border border-violet-500/20 bg-violet-500/5 rounded-lg p-3">
      <div className="flex items-center justify-between mb-2">
        <div className="text-[10px] font-mono uppercase tracking-[0.14em] text-text-2 flex items-center gap-1.5">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              offline ? 'bg-text-2' : 'bg-ok'
            }`}
            style={{ boxShadow: offline ? 'none' : '0 0 6px #84cc16' }}
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
            <div key={c.metric} className="bg-bg-0/40 rounded-md p-2">
              <div className="text-[9px] font-mono uppercase tracking-wider text-text-2 mb-1">
                {c.label}
              </div>
              <div
                className="font-mono font-bold leading-none mb-1"
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

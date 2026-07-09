import { useMetricsStore } from '../../lib/metricsStore'
import { SparkLine } from '../charts/SparkLine'
import { latestValue, ringToArrays } from '../../types/metrics'
import type { PeerMetric } from '../../lib/metricsStore'
import { COLORS } from '../../lib/colors'

const OFFLINE_AFTER_MS = 30_000

const CELLS: { metric: PeerMetric; label: string; color: string; fmt: (v: number) => string }[] = [
  { metric: 'cpu', label: 'CPU', color: COLORS.accent, fmt: (v) => `${Math.round(v)}%` },
  { metric: 'gpu', label: 'GPU', color: COLORS.accent, fmt: (v) => `${Math.round(v)}%` },
  { metric: 'ram', label: 'RAM free', color: COLORS.accent, fmt: (v) => `${v.toFixed(1)}G` },
  { metric: 'queue', label: 'Queue', color: COLORS.accent, fmt: (v) => `${Math.round(v)}` },
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
      <div className="border border-border-0 px-2 py-1.5 text-text-3 text-sm">
        Waiting for telemetry beacon…
      </div>
    )
  }

  const sinceLast = Date.now() - lastTick
  const offline = sinceLast > OFFLINE_AFTER_MS

  return (
    <div className="border border-border-0">
      <div className="flex items-center h-6 px-2 bg-chrome border-b border-border-0">
        <div className="text-2xs font-medium text-text-1 flex items-center gap-1.5 tabular-nums">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              offline ? 'bg-text-2' : 'bg-ok'
            }`}
          />
          {offline ? `Offline, last seen ${formatAgo(sinceLast)} ago` : 'Live telemetry, last 5 min'}
        </div>
      </div>
      <div className="grid grid-cols-4 divide-x divide-border-0">
        {CELLS.map((c) => {
          const buf = peerBufs.get(c.metric)!
          const v = latestValue(buf) ?? 0
          const { v: values } = ringToArrays(buf)
          return (
            <div key={c.metric} className="p-2">
              <div className="text-2xs font-medium text-text-2 mb-1">
                {c.label}
              </div>
              <div
                className="text-xs font-mono leading-none mb-1 tabular-nums"
                style={{ color: c.color }}
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

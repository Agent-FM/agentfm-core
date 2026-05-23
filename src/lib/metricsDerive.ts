import type { RingBuffer } from '../types/metrics'
import { latestValue, ringToArrays } from '../types/metrics'

export function computeRate(buf: RingBuffer): number {
  if (buf.filled < 2) return 0
  const { ts, v } = ringToArrays(buf)
  const dt = (ts[ts.length - 1] - ts[0]) / 1000
  if (dt <= 0) return 0
  const dv = v[v.length - 1] - v[0]
  if (dv < 0) return 0
  return dv / dt
}

export function computeTasksPerMinute(buf: RingBuffer): number {
  return computeRate(buf) * 60
}

export interface HistogramBucket {
  le: number
  count: number
}

export function computeP95FromBuckets(buckets: HistogramBucket[]): number {
  if (buckets.length === 0) return 0
  const sorted = [...buckets].sort((a, b) => a.le - b.le)
  const total = sorted[sorted.length - 1].count
  if (total <= 0) return 0
  const target = total * 0.95
  let prevLe = 0
  let prevCount = 0
  for (const b of sorted) {
    if (b.count >= target) {
      if (!Number.isFinite(b.le)) {
        const lastFinite = sorted
          .filter((x) => Number.isFinite(x.le))
          .map((x) => x.le)
          .pop()
        return lastFinite ?? 0
      }
      const bucketSize = b.count - prevCount
      if (bucketSize <= 0) return b.le
      const frac = (target - prevCount) / bucketSize
      return prevLe + frac * (b.le - prevLe)
    }
    prevLe = b.le
    prevCount = b.count
  }
  return sorted[sorted.length - 1].le
}

export { latestValue }

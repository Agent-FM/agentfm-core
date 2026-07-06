// src/types/metrics.ts

export type MetricType = 'counter' | 'gauge' | 'histogram' | 'summary' | 'unknown'

export interface MetricSample {
  name: string
  labels: Record<string, string>
  value: number
  type: MetricType
}

export interface RingBuffer {
  ts: Float64Array
  v: Float64Array
  head: number
  filled: number
}

export const RING_CAPACITY = 150

export function createRingBuffer(): RingBuffer {
  return {
    ts: new Float64Array(RING_CAPACITY),
    v: new Float64Array(RING_CAPACITY),
    head: 0,
    filled: 0,
  }
}

export function pushRing(buf: RingBuffer, ts: number, v: number): void {
  buf.ts[buf.head] = ts
  buf.v[buf.head] = v
  buf.head = (buf.head + 1) % RING_CAPACITY
  if (buf.filled < RING_CAPACITY) buf.filled++
}

export function latestValue(buf: RingBuffer): number | undefined {
  if (buf.filled === 0) return undefined
  const idx = (buf.head - 1 + RING_CAPACITY) % RING_CAPACITY
  return buf.v[idx]
}

export function ringToArrays(buf: RingBuffer): { ts: number[]; v: number[] } {
  if (buf.filled === 0) return { ts: [], v: [] }
  const ts: number[] = []
  const v: number[] = []
  const start = buf.filled < RING_CAPACITY ? 0 : buf.head
  for (let i = 0; i < buf.filled; i++) {
    const idx = (start + i) % RING_CAPACITY
    ts.push(buf.ts[idx])
    v.push(buf.v[idx])
  }
  return { ts, v }
}

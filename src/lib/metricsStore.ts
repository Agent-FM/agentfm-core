import { create } from 'zustand'
import {
  createRingBuffer,
  pushRing,
  latestValue,
  RingBuffer,
} from '../types/metrics'
import type { MetricSample } from '../types/metrics'

export type PeerMetric = 'cpu' | 'gpu' | 'ram' | 'queue'

export interface PeerSnapshot {
  cpu: number
  gpu: number
  ram: number
  queue: number
}

interface MetricsState {
  bossSeries: Map<string, RingBuffer>
  peerSeries: Map<string, Map<PeerMetric, RingBuffer>>
  peerLastTick: Map<string, number>
  lastBossTick: number
  pushBoss: (ts: number, samples: MetricSample[]) => void
  pushPeer: (peerId: string, ts: number, snap: PeerSnapshot) => void
  reset: () => void
}

export function seriesKey(name: string, labels: Record<string, string>): string {
  const keys = Object.keys(labels).sort()
  if (keys.length === 0) return name
  const pairs = keys.map((k) => `${k}=${labels[k]}`).join(',')
  return `${name}{${pairs}}`
}

export const useMetricsStore = create<MetricsState>((set, get) => ({
  bossSeries: new Map(),
  peerSeries: new Map(),
  peerLastTick: new Map(),
  lastBossTick: 0,

  pushBoss: (ts, samples) => {
    const { bossSeries } = get()
    const seen = new Set<string>()
    for (const s of samples) {
      const k = seriesKey(s.name, s.labels)
      let buf = bossSeries.get(k)
      if (!buf) {
        buf = createRingBuffer()
        bossSeries.set(k, buf)
      }
      pushRing(buf, ts, s.value)
      seen.add(k)
    }
    for (const [k, buf] of bossSeries) {
      if (seen.has(k)) continue
      const last = latestValue(buf)
      if (last !== undefined) pushRing(buf, ts, last)
    }
    set({ bossSeries, lastBossTick: ts })
  },

  pushPeer: (peerId, ts, snap) => {
    const { peerSeries, peerLastTick } = get()
    let peerBufs = peerSeries.get(peerId)
    if (!peerBufs) {
      peerBufs = new Map<PeerMetric, RingBuffer>([
        ['cpu', createRingBuffer()],
        ['gpu', createRingBuffer()],
        ['ram', createRingBuffer()],
        ['queue', createRingBuffer()],
      ])
      peerSeries.set(peerId, peerBufs)
    }
    pushRing(peerBufs.get('cpu')!, ts, snap.cpu)
    pushRing(peerBufs.get('gpu')!, ts, snap.gpu)
    pushRing(peerBufs.get('ram')!, ts, snap.ram)
    pushRing(peerBufs.get('queue')!, ts, snap.queue)
    peerLastTick.set(peerId, ts)
    set({
      peerSeries,
      peerLastTick,
    })
  },

  reset: () =>
    set({
      bossSeries: new Map(),
      peerSeries: new Map(),
      peerLastTick: new Map(),
      lastBossTick: 0,
    }),
}))

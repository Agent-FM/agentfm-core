import { describe, it, expect, beforeEach } from 'vitest'
import { useMetricsStore, seriesKey } from '../../src/lib/metricsStore'
import { latestValue, ringToArrays } from '../../src/types/metrics'

beforeEach(() => {
  useMetricsStore.getState().reset()
})

describe('metricsStore.pushBoss', () => {
  it('creates buffers on first push', () => {
    useMetricsStore.getState().pushBoss(1000, [
      { name: 'agentfm_tasks_total', labels: { status: 'ok' }, value: 5, type: 'counter' },
    ])
    const key = seriesKey('agentfm_tasks_total', { status: 'ok' })
    const buf = useMetricsStore.getState().bossSeries.get(key)
    expect(buf).toBeDefined()
    expect(latestValue(buf!)).toBe(5)
  })

  it('appends multiple ticks to the same series', () => {
    useMetricsStore.getState().pushBoss(1000, [{ name: 'g', labels: {}, value: 1, type: 'gauge' }])
    useMetricsStore.getState().pushBoss(2000, [{ name: 'g', labels: {}, value: 2, type: 'gauge' }])
    useMetricsStore.getState().pushBoss(3000, [{ name: 'g', labels: {}, value: 3, type: 'gauge' }])
    const buf = useMetricsStore.getState().bossSeries.get(seriesKey('g', {}))!
    expect(ringToArrays(buf)).toEqual({ ts: [1000, 2000, 3000], v: [1, 2, 3] })
  })

  it('carries forward series that are missing this tick', () => {
    useMetricsStore.getState().pushBoss(1000, [
      { name: 'a', labels: {}, value: 10, type: 'gauge' },
      { name: 'b', labels: {}, value: 20, type: 'gauge' },
    ])
    useMetricsStore.getState().pushBoss(2000, [
      { name: 'a', labels: {}, value: 11, type: 'gauge' },
    ])
    const { bossSeries } = useMetricsStore.getState()
    const b = bossSeries.get(seriesKey('b', {}))!
    expect(ringToArrays(b)).toEqual({ ts: [1000, 2000], v: [20, 20] })
  })
})

describe('metricsStore.pushPeer', () => {
  it('isolates per-peer buffers', () => {
    useMetricsStore.getState().pushPeer('peerA', 1000, { cpu: 50, gpu: 0, ram: 4, queue: 1 })
    useMetricsStore.getState().pushPeer('peerB', 1000, { cpu: 90, gpu: 0, ram: 2, queue: 3 })
    const { peerSeries } = useMetricsStore.getState()
    const a = peerSeries.get('peerA')!
    const b = peerSeries.get('peerB')!
    expect(latestValue(a.get('cpu')!)).toBe(50)
    expect(latestValue(b.get('cpu')!)).toBe(90)
    expect(latestValue(a.get('queue')!)).toBe(1)
    expect(latestValue(b.get('queue')!)).toBe(3)
  })
})

describe('seriesKey', () => {
  it('produces stable keys regardless of label insertion order', () => {
    const k1 = seriesKey('m', { a: '1', b: '2' })
    const k2 = seriesKey('m', { b: '2', a: '1' })
    expect(k1).toBe(k2)
  })
})

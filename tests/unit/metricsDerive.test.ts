import { describe, it, expect } from 'vitest'
import {
  computeRate,
  computeTasksPerMinute,
  computeP95FromBuckets,
} from '../../src/lib/metricsDerive'
import { createRingBuffer, pushRing } from '../../src/types/metrics'

describe('computeRate', () => {
  it('returns 0 when buffer has fewer than 2 points', () => {
    const b = createRingBuffer()
    expect(computeRate(b)).toBe(0)
    pushRing(b, 1000, 5)
    expect(computeRate(b)).toBe(0)
  })

  it('computes per-second rate between first and last samples', () => {
    const b = createRingBuffer()
    pushRing(b, 0, 0)
    pushRing(b, 1000, 10)
    pushRing(b, 2000, 30)
    expect(computeRate(b)).toBeCloseTo(15)
  })

  it('returns 0 when timestamps collide (avoid div-by-zero)', () => {
    const b = createRingBuffer()
    pushRing(b, 1000, 5)
    pushRing(b, 1000, 10)
    expect(computeRate(b)).toBe(0)
  })

  it('clamps negative rates (counter reset) to 0', () => {
    const b = createRingBuffer()
    pushRing(b, 0, 100)
    pushRing(b, 1000, 5)
    expect(computeRate(b)).toBe(0)
  })
})

describe('computeTasksPerMinute', () => {
  it('returns 0 on empty buffer', () => {
    expect(computeTasksPerMinute(createRingBuffer())).toBe(0)
  })

  it('returns rate × 60', () => {
    const b = createRingBuffer()
    pushRing(b, 0, 0)
    pushRing(b, 1000, 1)
    expect(computeTasksPerMinute(b)).toBeCloseTo(60)
  })
})

describe('computeP95FromBuckets', () => {
  it('returns 0 for empty buckets', () => {
    expect(computeP95FromBuckets([])).toBe(0)
  })

  it('returns 0 when total count is 0', () => {
    expect(computeP95FromBuckets([{ le: 1, count: 0 }, { le: Infinity, count: 0 }])).toBe(0)
  })

  it('interpolates p95 within the right bucket', () => {
    const p95 = computeP95FromBuckets([
      { le: 1, count: 10 },
      { le: 5, count: 80 },
      { le: 15, count: 95 },
      { le: 60, count: 98 },
      { le: Infinity, count: 100 },
    ])
    expect(p95).toBeCloseTo(15)
  })

  it('returns the highest finite bucket when p95 lands in +Inf', () => {
    const p95 = computeP95FromBuckets([
      { le: 1, count: 10 },
      { le: 60, count: 50 },
      { le: Infinity, count: 100 },
    ])
    expect(p95).toBe(60)
  })
})

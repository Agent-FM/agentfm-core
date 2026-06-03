// tests/unit/ringBuffer.test.ts
import { describe, it, expect } from 'vitest'
import {
  createRingBuffer,
  pushRing,
  latestValue,
  ringToArrays,
  RING_CAPACITY,
} from '../../src/types/metrics'

describe('RingBuffer', () => {
  it('starts empty', () => {
    const b = createRingBuffer()
    expect(b.filled).toBe(0)
    expect(latestValue(b)).toBeUndefined()
    expect(ringToArrays(b)).toEqual({ ts: [], v: [] })
  })

  it('pushes one value', () => {
    const b = createRingBuffer()
    pushRing(b, 100, 7)
    expect(b.filled).toBe(1)
    expect(latestValue(b)).toBe(7)
    expect(ringToArrays(b)).toEqual({ ts: [100], v: [7] })
  })

  it('preserves insertion order before wrap', () => {
    const b = createRingBuffer()
    for (let i = 0; i < 10; i++) pushRing(b, i, i * 2)
    const { ts, v } = ringToArrays(b)
    expect(ts).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9])
    expect(v).toEqual([0, 2, 4, 6, 8, 10, 12, 14, 16, 18])
    expect(latestValue(b)).toBe(18)
  })

  it('wraps at capacity and drops oldest', () => {
    const b = createRingBuffer()
    for (let i = 0; i < RING_CAPACITY + 5; i++) pushRing(b, i, i)
    const { ts, v } = ringToArrays(b)
    expect(ts.length).toBe(RING_CAPACITY)
    expect(v.length).toBe(RING_CAPACITY)
    expect(ts[0]).toBe(5) // oldest is push #5 (push 0..4 were overwritten)
    expect(v[v.length - 1]).toBe(RING_CAPACITY + 4) // newest
    expect(latestValue(b)).toBe(RING_CAPACITY + 4)
  })

  it('latestValue reflects most recent push after wrap', () => {
    const b = createRingBuffer()
    for (let i = 0; i < RING_CAPACITY * 2; i++) pushRing(b, i, i)
    expect(latestValue(b)).toBe(RING_CAPACITY * 2 - 1)
  })
})

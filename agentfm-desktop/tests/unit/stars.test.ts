import { describe, it, expect } from 'vitest'
import { starsFromScore } from '../../src/lib/stars'

describe('starsFromScore', () => {
  it('maps the -1..1 range onto 0..5 half-stars', () => {
    expect(starsFromScore(-1)).toBe(0)
    expect(starsFromScore(0)).toBe(2.5)
    expect(starsFromScore(1)).toBe(5)
    expect(starsFromScore(0.4)).toBe(3.5)
    expect(starsFromScore(-0.5)).toBe(1)
  })

  it('clamps out-of-range input', () => {
    expect(starsFromScore(5)).toBe(5)
    expect(starsFromScore(-5)).toBe(0)
  })
})

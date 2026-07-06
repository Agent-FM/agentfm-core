import { describe, it, expect } from 'vitest'
import { expoOut, entrance, lift, staggerItem, spring } from '../../src/lib/motion'

describe('motion tokens', () => {
  it('exposes the expo-out easing curve', () => {
    expect(expoOut).toEqual([0.16, 1, 0.3, 1])
  })
  it('entrance rises and fades in with expo-out', () => {
    expect(entrance.initial).toEqual({ opacity: 0, y: 6 })
    expect(entrance.animate).toEqual({ opacity: 1, y: 0 })
    expect(entrance.transition.ease).toEqual([0.16, 1, 0.3, 1])
  })
  it('lift taps to 0.97 via spring', () => {
    expect(lift.whileTap).toEqual({ scale: 0.97 })
    expect(lift.transition.type).toBe('spring')
  })
  it('staggerItem delays by 40ms per index', () => {
    expect(staggerItem(0).transition.delay).toBeCloseTo(0)
    expect(staggerItem(3).transition.delay).toBeCloseTo(0.12)
    expect(staggerItem(2).initial).toEqual({ opacity: 0, y: 8 })
  })
  it('still exports spring for existing callers', () => {
    expect(spring.type).toBe('spring')
  })
})

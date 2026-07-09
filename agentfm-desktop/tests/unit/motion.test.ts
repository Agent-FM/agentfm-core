import { describe, it, expect } from 'vitest'
import { easeOut, entrance, lift, staggerItem, spring } from '../../src/lib/motion'

describe('motion tokens (Xcode restraint: 150-200ms fades only)', () => {
  it('exposes the ease-out curve', () => {
    expect(easeOut).toEqual([0.25, 1, 0.5, 1])
  })
  it('entrance is a plain fast fade (no translate)', () => {
    expect(entrance.initial).toEqual({ opacity: 0 })
    expect(entrance.animate).toEqual({ opacity: 1 })
    expect(entrance.transition.duration).toBeLessThanOrEqual(0.2)
  })
  it('lift no longer scales or lifts (flat Xcode buttons)', () => {
    expect('whileTap' in lift).toBe(false)
    expect('whileHover' in lift).toBe(false)
    expect(lift.transition.duration).toBeLessThanOrEqual(0.2)
  })
  it('staggerItem has no per-index delay (no orchestrated entrances)', () => {
    expect(staggerItem(0).transition).not.toHaveProperty('delay')
    expect(staggerItem(20).transition).not.toHaveProperty('delay')
    expect(staggerItem(2).initial).toEqual({ opacity: 0 })
  })
  it('spring alias is duration-based and fast', () => {
    expect(spring.duration).toBeLessThanOrEqual(0.2)
  })
})

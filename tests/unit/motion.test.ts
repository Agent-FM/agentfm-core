import { describe, it, expect } from 'vitest'
import { fast, spring, entrance, lift } from '../../src/lib/motion'

describe('motion presets', () => {
  it('fast is a short ease-out', () => {
    expect(fast.duration).toBe(0.15)
    expect(fast.ease).toEqual([0.4, 0, 0.2, 1])
  })

  it('spring uses Framer-style config', () => {
    expect(spring.type).toBe('spring')
    expect(spring.stiffness).toBe(320)
    expect(spring.damping).toBe(28)
  })

  it('entrance has initial/animate/transition shape', () => {
    expect(entrance.initial).toEqual({ opacity: 0, y: 6 })
    expect(entrance.animate).toEqual({ opacity: 1, y: 0 })
    expect(entrance.transition.type).toBe('spring')
  })

  it('lift exposes whileHover and whileTap', () => {
    expect(lift.whileHover).toEqual({ y: -2 })
    expect(lift.whileTap).toEqual({ scale: 0.98 })
  })
})

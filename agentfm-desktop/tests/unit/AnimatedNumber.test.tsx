import { render, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AnimatedNumber } from '../../src/components/dashboard/AnimatedNumber'

function tick(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms)
  })
}

describe('AnimatedNumber', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the initial value immediately', () => {
    const { container } = render(<AnimatedNumber value={42} />)
    expect(container.textContent).toBe('42')
  })

  it('tweens between values when changed by less than 10x', () => {
    const { container, rerender } = render(<AnimatedNumber value={10} />)
    expect(container.textContent).toBe('10')

    rerender(<AnimatedNumber value={100} />)
    tick(250)
    const mid = Number(container.textContent)
    expect(mid).toBeGreaterThan(10)
    expect(mid).toBeLessThan(100)

    tick(500)
    expect(container.textContent).toBe('100')
  })

  it('skips the tween when the jump is >10x', () => {
    const { container, rerender } = render(<AnimatedNumber value={5} />)
    rerender(<AnimatedNumber value={1000} />)
    tick(16)
    expect(container.textContent).toBe('1000')
  })

  it('respects a custom format prop', () => {
    const { container } = render(
      <AnimatedNumber value={0.842} format={(n) => `${(n * 100).toFixed(1)}%`} />,
    )
    expect(container.textContent).toBe('84.2%')
  })
})

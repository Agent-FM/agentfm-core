import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Card } from '../../src/components/primitives/Card'

describe('Card', () => {
  it('is a frosted glass surface', () => {
    const { container } = render(<Card>x</Card>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/glass/)
    expect(cls).not.toMatch(/bg-bg-2/)
  })
  it('live state uses the accent-edged glass surface and a corner dot', () => {
    const { container } = render(<Card live>x</Card>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/glass-live/)
    expect(container.querySelector('[data-live-dot]')).toBeInTheDocument()
  })
  it('honors density padding', () => {
    const { container } = render(<Card density="compact">x</Card>)
    expect((container.firstChild as HTMLElement).className).toMatch(/p-2/)
  })
})

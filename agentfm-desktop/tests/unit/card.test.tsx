import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Card } from '../../src/components/primitives/Card'

describe('Card', () => {
  it('is a flat surface with hairline border and card shadow', () => {
    const { container } = render(<Card>x</Card>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/bg-bg-2/)
    expect(cls).toMatch(/border-border-0/)
    expect(cls).toMatch(/shadow-card/)
    expect(cls).not.toMatch(/glass/)
  })
  it('live state adds an accent ring and a corner dot', () => {
    const { container } = render(<Card live>x</Card>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/border-accent/)
    expect(container.querySelector('[data-live-dot]')).toBeInTheDocument()
  })
  it('honors density padding', () => {
    const { container } = render(<Card density="compact">x</Card>)
    expect((container.firstChild as HTMLElement).className).toMatch(/p-3/)
  })
})

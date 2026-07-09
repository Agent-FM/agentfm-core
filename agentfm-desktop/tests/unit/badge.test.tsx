import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Badge } from '../../src/components/primitives/Badge'

describe('Badge', () => {
  it('renders children', () => {
    const { getByText } = render(<Badge>hello</Badge>)
    expect(getByText('hello')).toBeTruthy()
  })

  it('applies cyan tone classes by default', () => {
    const { container } = render(<Badge>x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-accent/)
  })

  it('applies bad tone classes when tone=bad', () => {
    const { container } = render(<Badge tone="bad">x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-bad/)
  })

  it('renders mono when mono=true', () => {
    const { container } = render(<Badge mono>x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/font-mono/)
  })

  it('renders neutral tone with secondary text', () => {
    const { container } = render(<Badge tone="neutral">x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-text-1/)
  })
})

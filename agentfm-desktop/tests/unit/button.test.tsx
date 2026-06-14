import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Button } from '../../src/components/primitives/Button'

describe('Button', () => {
  it('primary uses the solid accent background and dark fg', () => {
    const { container } = render(<Button variant="primary">Go</Button>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/bg-accent\b/)
    expect(cls).toMatch(/text-accent-fg/)
    expect(cls).not.toMatch(/gradient/)
  })
  it('secondary uses a surface + hairline border', () => {
    const { container } = render(<Button variant="secondary">Go</Button>)
    const cls = (container.firstChild as HTMLElement).className
    expect(cls).toMatch(/bg-bg-1/)
    expect(cls).toMatch(/border-border-0/)
  })
  it('danger uses the bad token', () => {
    const { container } = render(<Button variant="danger">Del</Button>)
    expect((container.firstChild as HTMLElement).className).toMatch(/text-bad/)
  })
  it('renders children and respects disabled', () => {
    const { getByText } = render(<Button disabled>Hi</Button>)
    expect((getByText('Hi') as HTMLButtonElement).disabled).toBe(true)
  })
})

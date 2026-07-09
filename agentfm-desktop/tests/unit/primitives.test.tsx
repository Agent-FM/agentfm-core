import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'

afterEach(() => cleanup())
import { RoutePage } from '../../src/components/primitives/RoutePage'
import { SectionLabel } from '../../src/components/primitives/SectionLabel'
import { HeroTitle } from '../../src/components/primitives/HeroTitle'
import { Avatar } from '../../src/components/primitives/Avatar'
import { Meter } from '../../src/components/primitives/Meter'

describe('RoutePage', () => {
  it('renders content on a plain solid surface with no ambient decor', () => {
    const { container } = render(<RoutePage><div>hello</div></RoutePage>)
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(container.querySelectorAll('.animate-aurora').length).toBe(0)
    expect(container.querySelector('.route-page__grid')).toBeNull()
  })
})

describe('SectionLabel', () => {
  it('renders the label as a small caption', () => {
    render(<SectionLabel>Backend</SectionLabel>)
    const el = screen.getByText('Backend')
    expect(el.className).toMatch(/text-2xs/)
    expect(el.className).toMatch(/font-medium/)
  })
  it('applies text-bad when tone is bad', () => {
    render(<SectionLabel tone="bad">DANGER ZONE</SectionLabel>)
    const el = screen.getByText('DANGER ZONE')
    expect(el.className).toMatch(/text-bad/)
  })
})

describe('HeroTitle', () => {
  it('wraps the accent word in an accent span, no shimmer', () => {
    const { container } = render(<HeroTitle accent="mesh">Your</HeroTitle>)
    const span = container.querySelector('[data-hero-accent]') as HTMLElement
    expect(span.textContent).toBe('mesh')
    expect(span.className).toMatch(/text-accent/)
    expect(container.querySelector('.hero-shimmer')).toBeNull()
  })
})

describe('Avatar', () => {
  it('renders an emoji without a rotating halo', () => {
    const { container } = render(<Avatar emoji="🤖" />)
    expect(screen.getByText('🤖')).toBeInTheDocument()
    expect(container.querySelector('.animate-halo-rotate')).toBeNull()
  })
  it('falls back to children when no emoji is given', () => {
    render(<Avatar>HR</Avatar>)
    expect(screen.getByText('HR')).toBeInTheDocument()
  })
})

describe('Meter', () => {
  it('clamps the fill width and ramps orange → amber past 70 → red past 90', () => {
    const { container, rerender } = render(<Meter value={50} />)
    const fill = container.querySelector<HTMLElement>('[data-meter-fill]')!
    expect(fill.style.width).toBe('50%')
    expect(fill.className).toMatch(/bg-accent/)
    expect(container.querySelector('.animate-meter-shimmer')).toBeNull()
    rerender(<Meter value={80} />)
    expect(fill.className).toMatch(/bg-warn/)
    rerender(<Meter value={150} />)
    expect(fill.style.width).toBe('100%')
    expect(fill.className).toMatch(/bg-bad/)
    rerender(<Meter value={-20} />)
    expect(fill.style.width).toBe('0%')
  })
})

import { ProjectChip } from '../../src/components/primitives/ProjectChip'
import { RelayPill } from '../../src/components/primitives/RelayPill'

describe('ProjectChip', () => {
  it('renders the project name and a static status dot (Xcode restraint)', () => {
    const { container } = render(<ProjectChip name="Private Workspace" />)
    expect(screen.getByText('Private Workspace')).toBeInTheDocument()
    expect(container.querySelector('.bg-accent.rounded-full')).toBeInTheDocument()
  })
})

describe('RelayPill', () => {
  it('renders a shortened peer id with a lock icon when private', () => {
    const { container } = render(<RelayPill peerId="12D3KooWPorLn55wwUdnBCipJMe4sFLUJEAESexeVtzYGTiTWw68" mode="private" />)
    expect(container.querySelector('svg.lucide-lock')).toBeInTheDocument()
    expect(screen.getByText(/12D3Ko/)).toBeInTheDocument()
  })
  it('omits the lock icon when public', () => {
    const { container } = render(<RelayPill peerId="12D3KooWPorLn55wwUdnBCipJMe4sFLUJEAESexeVtzYGTiTWw68" mode="public" />)
    expect(container.querySelector('svg.lucide-lock')).toBeNull()
  })
})

import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'

afterEach(() => cleanup())
import { RoutePage } from '../../src/components/primitives/RoutePage'
import { SectionLabel } from '../../src/components/primitives/SectionLabel'
import { HeroTitle } from '../../src/components/primitives/HeroTitle'
import { Avatar } from '../../src/components/primitives/Avatar'
import { Meter } from '../../src/components/primitives/Meter'

describe('RoutePage', () => {
  it('renders content with two calm ambient blobs and no mesh grid', () => {
    const { container } = render(<RoutePage><div>hello</div></RoutePage>)
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(container.querySelectorAll('.animate-aurora').length).toBe(2)
    expect(container.querySelector('.route-page__grid')).toBeNull()
  })
})

describe('SectionLabel', () => {
  it('renders the label with the ▌ glyph', () => {
    render(<SectionLabel>BACKEND</SectionLabel>)
    const el = screen.getByText(/BACKEND/)
    expect(el.textContent).toContain('▌')
  })
})

describe('HeroTitle', () => {
  it('wraps the accent word in a shimmer span', () => {
    const { container } = render(<HeroTitle accent="mesh">Your</HeroTitle>)
    expect(container.querySelector('.hero-shimmer')).toBeInTheDocument()
    expect(container.querySelector('.hero-shimmer')!.textContent).toBe('mesh')
  })
})

describe('Avatar', () => {
  it('renders the emoji prop', () => {
    render(<Avatar emoji="🤖" />)
    expect(screen.getByText('🤖')).toBeInTheDocument()
  })
  it('always includes the rotating conic halo', () => {
    const { container } = render(<Avatar emoji="🤖" />)
    expect(container.querySelector('.animate-halo-rotate')).toBeInTheDocument()
  })
})

describe('Meter', () => {
  it('renders a flat accent fill at the clamped width, no shimmer', () => {
    const { container, rerender } = render(<Meter value={150} />)
    const fill = container.querySelector<HTMLElement>('[data-meter-fill]')!
    expect(fill.style.width).toBe('100%')
    expect(fill.className).toMatch(/bg-accent/)
    expect(container.querySelector('.animate-meter-shimmer')).toBeNull()
    rerender(<Meter value={-20} />)
    expect(fill.style.width).toBe('0%')
  })
})

import { ProjectChip } from '../../src/components/primitives/ProjectChip'
import { RelayPill } from '../../src/components/primitives/RelayPill'

describe('ProjectChip', () => {
  it('renders the project name and a pulsing dot', () => {
    const { container } = render(<ProjectChip name="Private Workspace" />)
    expect(screen.getByText('Private Workspace')).toBeInTheDocument()
    expect(container.querySelector('.animate-pulse-cyan')).toBeInTheDocument()
  })
})

describe('RelayPill', () => {
  it('renders a shortened peer id with the lock prefix when private', () => {
    render(<RelayPill peerId="12D3KooWPorLn55wwUdnBCipJMe4sFLUJEAESexeVtzYGTiTWw68" mode="private" />)
    expect(screen.getByText(/🔒/)).toBeInTheDocument()
    expect(screen.getByText(/12D3Ko/)).toBeInTheDocument()
  })
  it('omits the lock prefix when public', () => {
    render(<RelayPill peerId="12D3KooWPorLn55wwUdnBCipJMe4sFLUJEAESexeVtzYGTiTWw68" mode="public" />)
    expect(screen.queryByText(/🔒/)).toBeNull()
  })
})

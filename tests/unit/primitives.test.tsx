import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoutePage } from '../../src/components/primitives/RoutePage'
import { SectionLabel } from '../../src/components/primitives/SectionLabel'
import { HeroTitle } from '../../src/components/primitives/HeroTitle'
import { NeonCard } from '../../src/components/primitives/NeonCard'
import { Avatar } from '../../src/components/primitives/Avatar'
import { Meter } from '../../src/components/primitives/Meter'

describe('RoutePage', () => {
  it('renders content and the two mesh background layers', () => {
    const { container } = render(<RoutePage><div>hello</div></RoutePage>)
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(container.querySelector('.route-page__blobs')).toBeInTheDocument()
    expect(container.querySelector('.route-page__grid')).toBeInTheDocument()
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

describe('NeonCard', () => {
  it('renders children and the top-sweep light bar always', () => {
    const { container } = render(<NeonCard>hi</NeonCard>)
    expect(screen.getByText('hi')).toBeInTheDocument()
    expect(container.querySelector('.animate-top-sweep')).toBeInTheDocument()
  })
  it('adds glow-cycle when breathing', () => {
    const { container } = render(<NeonCard breathing>hi</NeonCard>)
    expect(container.querySelector('.animate-glow-cycle')).toBeInTheDocument()
  })
  it('omits glow-cycle when not breathing', () => {
    const { container } = render(<NeonCard>hi</NeonCard>)
    expect(container.querySelector('.animate-glow-cycle')).toBeNull()
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
  it('clamps value to 0..100', () => {
    const { container, rerender } = render(<Meter value={150} />)
    const fill = container.querySelector<HTMLElement>('[data-meter-fill]')!
    expect(fill.style.width).toBe('100%')
    rerender(<Meter value={-20} />)
    expect(fill.style.width).toBe('0%')
  })
  it('renders the shimmer streak', () => {
    const { container } = render(<Meter value={50} />)
    expect(container.querySelector('.animate-meter-shimmer')).toBeInTheDocument()
  })
})

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RoutePage } from '../../src/components/primitives/RoutePage'
import { SectionLabel } from '../../src/components/primitives/SectionLabel'
import { HeroTitle } from '../../src/components/primitives/HeroTitle'

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

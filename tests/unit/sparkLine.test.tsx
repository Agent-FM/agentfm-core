import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { SparkLine } from '../../src/components/charts/SparkLine'

describe('SparkLine', () => {
  it('renders a canvas element', () => {
    const { container } = render(
      <SparkLine values={[1, 2, 3, 4]} width={120} height={30} color="#22d3ee" />,
    )
    const canvas = container.querySelector('canvas')
    expect(canvas).toBeTruthy()
  })

  it('renders an empty canvas for empty values', () => {
    const { container } = render(
      <SparkLine values={[]} width={120} height={30} color="#22d3ee" />,
    )
    const canvas = container.querySelector('canvas')
    expect(canvas).toBeTruthy()
  })

  it('renders for a single value', () => {
    const { container } = render(
      <SparkLine values={[42]} width={120} height={30} color="#22d3ee" />,
    )
    expect(container.querySelector('canvas')).toBeTruthy()
  })
})

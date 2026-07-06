import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { GettingStarted } from '../../src/components/developer/GettingStarted'

describe('GettingStarted', () => {
  it('shows the base url and an OpenAI SDK example', () => {
    const { getByText, container } = render(<GettingStarted baseURL="http://127.0.0.1:8080" authEnabled={false} />)
    expect(getByText('http://127.0.0.1:8080')).toBeTruthy()
    expect(container.textContent).toContain('base_url="http://127.0.0.1:8080/v1"')
  })

  it('notes auth is off when authEnabled is false', () => {
    const { container } = render(<GettingStarted baseURL="http://127.0.0.1:8080" authEnabled={false} />)
    expect(container.textContent?.toLowerCase()).toContain('no api key')
  })
})

import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { ResponsePanel } from '../../src/components/developer/ResponsePanel'

describe('ResponsePanel', () => {
  it('renders status, timing and pretty JSON for a success', () => {
    const { container } = render(
      <ResponsePanel result={{ ok: true, status: 200, ms: 12, body: '{"a":1}', contentType: 'application/json' }} loading={false} />,
    )
    expect(container.textContent).toContain('200')
    expect(container.textContent).toContain('12 ms')
    expect(container.textContent).toContain('"a": 1')
  })

  it('renders an error state when status is 0', () => {
    const { container } = render(
      <ResponsePanel result={{ ok: false, status: 0, ms: 3, body: '', contentType: '', error: 'failed to fetch' }} loading={false} />,
    )
    expect(container.textContent?.toLowerCase()).toContain('not reachable')
  })

  it('shows a placeholder before any request', () => {
    const { container } = render(<ResponsePanel result={null} loading={false} />)
    expect(container.textContent?.toLowerCase()).toContain('send a request')
  })
})

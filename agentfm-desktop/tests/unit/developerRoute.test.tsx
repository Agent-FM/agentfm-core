import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import Developer from '../../src/routes/Developer'

describe('Developer route', () => {
  it('renders heading, getting-started base url, and the endpoint list', () => {
    const { getByText, getAllByText, container } = render(<Developer />)
    expect(getByText('Developer API')).toBeTruthy()
    expect(container.textContent).toContain('http://127.0.0.1')
    expect(getAllByText(/\/api\/workers/).length).toBeGreaterThan(0)
  })
})

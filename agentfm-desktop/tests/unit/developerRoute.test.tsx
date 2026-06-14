import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import Developer from '../../src/routes/Developer'

describe('Developer route', () => {
  it('renders the developer heading and lists a known endpoint', () => {
    const { getAllByText } = render(<Developer />)
    expect(getAllByText(/Developer/i).length).toBeGreaterThan(0)
    // catalog-driven: /api/workers should appear in the endpoint list
    expect(getAllByText(/\/api\/workers/).length).toBeGreaterThan(0)
  })
})

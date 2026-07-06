import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { EndpointList } from '../../src/components/developer/EndpointList'
import { API_CATALOG } from '../../src/lib/apiCatalog'

describe('EndpointList', () => {
  it('groups endpoints and calls onSelect with the endpoint id', () => {
    const onSelect = vi.fn()
    const { getByText } = render(
      <EndpointList endpoints={API_CATALOG} selectedId="health" onSelect={onSelect} />,
    )
    expect(getByText('OpenAI-compatible')).toBeTruthy()
    fireEvent.click(getByText('/api/workers'))
    expect(onSelect).toHaveBeenCalledWith('workers')
  })
})

import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { RequestForm } from '../../src/components/developer/RequestForm'
import { API_CATALOG } from '../../src/lib/apiCatalog'

const workers = API_CATALOG.find((e) => e.id === 'workers')!
const execute = API_CATALOG.find((e) => e.id === 'execute')!

describe('RequestForm', () => {
  it('renders a query input and reports changes', () => {
    const onChange = vi.fn()
    const { getByLabelText } = render(
      <RequestForm endpoint={workers} values={{}} onChange={onChange} />,
    )
    fireEvent.change(getByLabelText('include_offline'), { target: { value: 'true' } })
    expect(onChange).toHaveBeenCalledWith({ include_offline: 'true' })
  })

  it('renders a JSON body textarea for body endpoints', () => {
    const { getByLabelText } = render(
      <RequestForm endpoint={execute} values={{}} onChange={() => {}} />,
    )
    const ta = getByLabelText('Request body (JSON)') as HTMLTextAreaElement
    expect(ta.tagName).toBe('TEXTAREA')
    expect(ta.value).toContain('prompt')
  })
})

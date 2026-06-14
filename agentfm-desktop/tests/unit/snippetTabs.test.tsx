import { describe, it, expect } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { SnippetTabs } from '../../src/components/developer/SnippetTabs'
import { API_CATALOG } from '../../src/lib/apiCatalog'

const chat = API_CATALOG.find((e) => e.id === 'chat-completions')!
const workers = API_CATALOG.find((e) => e.id === 'workers')!

describe('SnippetTabs', () => {
  it('defaults to curl and switches language on click', () => {
    const { getByText, container } = render(
      <SnippetTabs endpoint={chat} values={{}} baseURL="http://127.0.0.1:8080" />,
    )
    expect(container.textContent).toContain('curl -X POST')
    fireEvent.click(getByText('Python'))
    expect(container.textContent).toContain('import requests')
  })

  it('hides the OpenAI SDK tab for non-OpenAI endpoints', () => {
    const { queryByText } = render(
      <SnippetTabs endpoint={workers} values={{}} baseURL="http://127.0.0.1:8080" />,
    )
    expect(queryByText('OpenAI SDK')).toBeNull()
  })
})

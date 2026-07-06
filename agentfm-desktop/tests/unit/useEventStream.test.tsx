import { it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'

const openEventStream = vi.fn(() => ({ close: vi.fn() }))
vi.mock('../../src/lib/sse', () => ({
  openEventStream: (...args: unknown[]) => openEventStream(...args),
}))

import { useEventStream } from '../../src/hooks/useEventStream'
import { setApiPort } from '../../src/lib/api'

it('connects the SSE stream to the configured API port, not a hardcoded one', () => {
  setApiPort(9123)
  const qc = new QueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
  renderHook(() => useEventStream(), { wrapper })

  expect(openEventStream).toHaveBeenCalledWith(
    'http://127.0.0.1:9123/v1/events',
    expect.anything(),
  )
})

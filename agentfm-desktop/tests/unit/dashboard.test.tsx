import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import Dashboard from '../../src/routes/Dashboard'
import { useMetricsStore } from '../../src/lib/metricsStore'

beforeEach(() => {
  useMetricsStore.getState().reset()
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve('') }),
  )
})

describe('Dashboard', () => {
  it('renders without crashing on empty store', () => {
    const { container } = render(<Dashboard />)
    expect(container.textContent).toMatch(/TASKS/i)
  })

  it('renders the hero tile with the latest task count when data is seeded', () => {
    useMetricsStore.getState().pushBoss(Date.now() - 1000, [
      { name: 'agentfm_tasks_total', labels: { status: 'ok' }, value: 100, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'error' }, value: 2, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'rejected' }, value: 0, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'timeout' }, value: 0, type: 'counter' },
    ])
    useMetricsStore.getState().pushBoss(Date.now(), [
      { name: 'agentfm_tasks_total', labels: { status: 'ok' }, value: 142, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'error' }, value: 3, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'rejected' }, value: 0, type: 'counter' },
      { name: 'agentfm_tasks_total', labels: { status: 'timeout' }, value: 1, type: 'counter' },
    ])
    const { container } = render(<Dashboard />)
    expect(container.textContent).toMatch(/146/)
  })
})

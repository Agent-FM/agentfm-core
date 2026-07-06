import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useMetricsPoll } from '../../src/hooks/useMetricsPoll'
import { useMetricsStore } from '../../src/lib/metricsStore'

beforeEach(() => {
  useMetricsStore.getState().reset()
  vi.useFakeTimers()
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    get: () => 'visible',
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useMetricsPoll', () => {
  it('fetches /metrics on mount and pushes parsed samples', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        text: () => Promise.resolve('# TYPE x gauge\nx 5'),
      }),
    )
    renderHook(() => useMetricsPoll())
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(fetch).toHaveBeenCalledTimes(1)
    const bossSeries = useMetricsStore.getState().bossSeries
    expect(bossSeries.size).toBeGreaterThan(0)
  })

  it('keeps fast 2s cadence after a single successful fetch', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        text: () => Promise.resolve(''),
      }),
    )
    renderHook(() => useMetricsPoll())
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('backs off to 10s after 3 consecutive errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')))
    renderHook(() => useMetricsPoll())
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(fetch).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetch).toHaveBeenCalledTimes(2)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetch).toHaveBeenCalledTimes(3)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetch).toHaveBeenCalledTimes(3)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(8_000)
    })
    expect(fetch).toHaveBeenCalledTimes(4)
  })

  it('returns to fast cadence after first success following backoff', async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new Error('boom'))
      .mockRejectedValueOnce(new Error('boom'))
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValue({ ok: true, text: () => Promise.resolve('') })
    vi.stubGlobal('fetch', fetchMock)
    renderHook(() => useMetricsPoll())
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(3)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(4)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(5)
  })

  it('does not fetch while document is hidden', async () => {
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'hidden',
    })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve('') }),
    )
    renderHook(() => useMetricsPoll())
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(fetch).not.toHaveBeenCalled()
  })
})

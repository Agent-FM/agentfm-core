import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock fetch globally before importing the module under test
const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// window.api is not present in node test env; the module only reads it at
// loadApiPortFromSettings() call time, so we just need the global to exist.
vi.stubGlobal('window', { api: undefined })

import { api, ApiError } from '../../src/lib/api'

function makeJsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : 'Error',
    json: async () => body,
    text: async () => JSON.stringify(body),
  }
}

describe('api.workers', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  it('calls /api/workers without query string when includeOffline=false', async () => {
    mockFetch.mockResolvedValue(
      makeJsonResponse({ success: true, online_count: 2, offline_count: 0, agents: [] }),
    )

    await api.workers()

    expect(mockFetch).toHaveBeenCalledOnce()
    const [url] = mockFetch.mock.calls[0] as [string]
    expect(url).toMatch(/\/api\/workers$/)
    expect(url).not.toContain('include_offline')
  })

  it('appends ?include_offline=true when includeOffline=true', async () => {
    mockFetch.mockResolvedValue(
      makeJsonResponse({ success: true, online_count: 2, offline_count: 3, agents: [] }),
    )

    await api.workers(true)

    const [url] = mockFetch.mock.calls[0] as [string]
    expect(url).toContain('?include_offline=true')
  })
})

describe('api.about', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  it('calls /v1/about', async () => {
    mockFetch.mockResolvedValue(
      makeJsonResponse({
        boss_peer_id: 'peer1',
        relay_peer_id: 'peer2',
        relay_multiaddr: '/ip4/127.0.0.1',
        reputation_floor: -0.5,
        ledger_tree_size: 42,
        version: '1.2.0',
        uptime_seconds: 100,
      }),
    )

    const result = await api.about()
    const [url] = mockFetch.mock.calls[0] as [string]
    expect(url).toMatch(/\/v1\/about$/)
    expect(result.version).toBe('1.2.0')
  })
})

describe('ApiError', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  it('throws ApiError on non-2xx response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      text: async () => 'peer not found',
    })

    await expect(api.peer('nonexistent')).rejects.toThrow(ApiError)
  })

  it('ApiError has correct status code', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      text: async () => 'internal error',
    })

    let caught: ApiError | undefined
    try {
      await api.about()
    } catch (e) {
      caught = e as ApiError
    }
    expect(caught).toBeInstanceOf(ApiError)
    expect(caught!.status).toBe(500)
    expect(caught!.message).toBe('internal error')
  })

  it('falls back to statusText when body is empty', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      text: async () => '',
    })

    let caught: ApiError | undefined
    try {
      await api.health()
    } catch (e) {
      caught = e as ApiError
    }
    expect(caught!.message).toBe('Service Unavailable')
  })
})

describe('api.submitSelfComment', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  it('POSTs to /v1/peers/{id}/comments/self with JSON body', async () => {
    mockFetch.mockResolvedValue(
      makeJsonResponse({ cid: 'abc123', ledger_hash: 'def456' }, 201),
    )

    const result = await api.submitSelfComment('peer-xyz', {
      text: 'good work',
      language: 'en',
    })

    expect(mockFetch).toHaveBeenCalledOnce()
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit]
    expect(url).toMatch(/\/v1\/peers\/peer-xyz\/comments\/self$/)
    expect(init.method).toBe('POST')
    expect(init.headers).toMatchObject({ 'Content-Type': 'application/json' })
    expect(JSON.parse(init.body as string)).toEqual({ text: 'good work', language: 'en' })
    expect(result.cid).toBe('abc123')
    expect(result.ledger_hash).toBe('def456')
  })

  it('throws ApiError on non-2xx', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: async () => 'text is required',
    })

    await expect(
      api.submitSelfComment('peer-xyz', { text: '' }),
    ).rejects.toThrow(ApiError)
  })
})

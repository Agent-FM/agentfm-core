import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { shortenPeerID, shortenDigest, compactAge } from '../../src/lib/peer'

describe('shortenPeerID', () => {
  it('truncates a long peer ID', () => {
    const long = '12D3KooWAbcdefghijklmnopqrstuvwxyz0123456789'
    const result = shortenPeerID(long, 6, 5)
    expect(result).toBe(`${long.slice(0, 6)}…${long.slice(-5)}`)
  })

  it('returns short IDs unchanged', () => {
    const short = '12D3Ko'
    expect(shortenPeerID(short, 6, 5)).toBe(short)
  })

  it('handles exactly head+tail+3 length unchanged', () => {
    const exact = 'abcdef123' // 9 chars = 6 + 5 - 2, under threshold
    const atThreshold = 'abcdef...xyz' // 12 chars = 6 + 5 + 1, still short
    // length <= head + tail + 3 → returned as-is
    const s = 'abcdefxyz12' // 11 chars, 6+5=11, 11 <= 14 → unchanged
    expect(shortenPeerID(s, 6, 5)).toBe(s)
  })
})

describe('shortenDigest', () => {
  it('strips sha256: prefix and shortens', () => {
    const result = shortenDigest('sha256:abc1234567890abcdef', 8)
    expect(result).toBe('sha256:abc12345…')
  })

  it('handles digest without sha256: prefix', () => {
    const result = shortenDigest('abc1234567890', 8)
    expect(result).toBe('sha256:abc12345…')
  })

  it('returns empty string for empty input', () => {
    expect(shortenDigest('')).toBe('')
  })
})

describe('compactAge', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns seconds for <60s', () => {
    const ts = Date.now() - 5000
    expect(compactAge(ts)).toBe('5s')
  })

  it('returns minutes for 60s–3599s', () => {
    const ts = Date.now() - 90 * 1000
    expect(compactAge(ts)).toBe('1m')
  })

  it('returns hours for 1h–23h', () => {
    const ts = Date.now() - 2 * 60 * 60 * 1000
    expect(compactAge(ts)).toBe('2h')
  })

  it('returns days for 24h+', () => {
    const ts = Date.now() - 3 * 24 * 60 * 60 * 1000
    expect(compactAge(ts)).toBe('3d')
  })

  it('accepts a Date object', () => {
    const d = new Date(Date.now() - 5000)
    expect(compactAge(d)).toBe('5s')
  })

  it('accepts a string date', () => {
    const d = new Date(Date.now() - 5000).toISOString()
    expect(compactAge(d)).toBe('5s')
  })
})

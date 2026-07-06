/** @vitest-environment node */

import { describe, it, expect } from 'vitest'
import { isValidMultiaddr } from '../../electron/validate'

describe('isValidMultiaddr', () => {
  it('accepts real multiaddrs', () => {
    expect(isValidMultiaddr('/ip4/127.0.0.1/tcp/4015/p2p/12D3KooWabc')).toBe(true)
    expect(isValidMultiaddr('/ip4/1.2.3.4/tcp/4015')).toBe(true)
    expect(isValidMultiaddr('/dns4/relay.example.com/tcp/443/p2p/12D3KooWabc')).toBe(true)
    expect(isValidMultiaddr('/dns4/relay.example/tcp/443/p2p/12D3KooWabc')).toBe(true)
    expect(isValidMultiaddr('/ip6/::1/tcp/4015/p2p/12D3KooWabc')).toBe(true)
  })

  it('accepts null and empty string (to clear the setting)', () => {
    expect(isValidMultiaddr(null)).toBe(true)
    expect(isValidMultiaddr('')).toBe(true)
    expect(isValidMultiaddr(undefined)).toBe(true)
  })

  it('rejects injection payloads and malformed values', () => {
    expect(isValidMultiaddr('/ip4/1.2.3.4/tcp/1 -bootstrap /evil')).toBe(false)
    expect(isValidMultiaddr('; rm -rf ~')).toBe(false)
    expect(isValidMultiaddr('/ip4/1.2.3.4/tcp/1$(whoami)')).toBe(false)
    expect(isValidMultiaddr('ip4/127.0.0.1/tcp/4015')).toBe(false)
    expect(isValidMultiaddr('/ip4/1.2.3.4/tcp/1 -someflag')).toBe(false)
    expect(isValidMultiaddr('--bootstrap')).toBe(false)
    expect(isValidMultiaddr(42)).toBe(false)
    expect(isValidMultiaddr(true)).toBe(false)
    expect(isValidMultiaddr('/path with spaces/tcp/1')).toBe(false)
  })
})

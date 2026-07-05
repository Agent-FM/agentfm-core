import { describe, it, expect, beforeEach } from 'vitest'
import { usePeerIdentityCache } from '../../src/lib/peerIdentityCache'

describe('peerIdentityCache persistence', () => {
  beforeEach(() => {
    localStorage.clear()
    usePeerIdentityCache.setState({ byPeerId: {} })
  })

  it('writes remembered identities to localStorage', () => {
    usePeerIdentityCache.getState().remember([
      { peer_id: 'p1', name: 'HR Agent', online: true } as never,
    ])
    const raw = localStorage.getItem('agentfm-peer-identity')
    expect(raw).toBeTruthy()
    expect(raw as string).toContain('HR Agent')
  })
})

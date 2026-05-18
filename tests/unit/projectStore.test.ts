import { describe, it, expect } from 'vitest'
import {
  newProjectId,
  validateProjectInput,
  DuplicateRelayError,
} from '../../src/lib/projectStore'
import type { Project } from '../../src/types/project'

const base = (overrides: Partial<Project> = {}): Project => ({
  id: 'prj_test',
  name: 'Default',
  relayMultiaddr: null,
  reputationFloor: -0.5,
  createdAt: 0,
  ...overrides,
})

describe('newProjectId', () => {
  it('returns prj_-prefixed ids', () => {
    const id = newProjectId()
    expect(id).toMatch(/^prj_[a-z0-9]{8}$/)
  })
  it('returns distinct ids on repeated calls', () => {
    expect(newProjectId()).not.toBe(newProjectId())
  })
})

describe('validateProjectInput', () => {
  it('passes when name is set and relay is unique', () => {
    expect(() =>
      validateProjectInput([base({ relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Test' })],
        { name: 'Two', relayMultiaddr: '/ip4/5.6.7.8/tcp/4001/p2p/12D3Other' }),
    ).not.toThrow()
  })

  it('rejects empty name', () => {
    expect(() => validateProjectInput([], { name: '   ', relayMultiaddr: null })).toThrow(
      /name is required/i,
    )
  })

  it('throws DuplicateRelayError when relay is already used', () => {
    expect(() =>
      validateProjectInput(
        [base({ relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Same' })],
        { name: 'New', relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Same' },
      ),
    ).toThrow(DuplicateRelayError)
  })

  it('treats null relays as one slot only', () => {
    expect(() =>
      validateProjectInput([base({ relayMultiaddr: null })], {
        name: 'Second default',
        relayMultiaddr: null,
      }),
    ).toThrow(DuplicateRelayError)
  })

  it('allows null when editing the same project that already owns null', () => {
    expect(() =>
      validateProjectInput(
        [base({ id: 'prj_keep', relayMultiaddr: null })],
        { name: 'Default renamed', relayMultiaddr: null },
        'prj_keep',
      ),
    ).not.toThrow()
  })

  it('rejects out-of-range reputationFloor', () => {
    expect(() =>
      validateProjectInput([], {
        name: 'x',
        relayMultiaddr: null,
        reputationFloor: 0.5,
      }),
    ).toThrow(/reputation floor/i)
  })
})

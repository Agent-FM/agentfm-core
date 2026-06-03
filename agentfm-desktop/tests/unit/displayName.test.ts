import { describe, it, expect } from 'vitest'
import { displayName } from '../../src/lib/displayName'

const peer = '12D3KooWMP2KCh1qKk6PPw8oH6GXUwRYjMYEbGYdzMZ5fygdt4Es'

describe('displayName', () => {
  it('prefers explicit name', () => {
    expect(
      displayName({
        peer_id: peer,
        name: 'HR Specialist',
        agent_capability: 'hr',
        agent_image_ref: 'ghcr.io/yourorg/hr-agent:v1',
      }),
    ).toBe('HR Specialist')
  })

  it('trims whitespace-only names and falls back to capability', () => {
    expect(
      displayName({
        peer_id: peer,
        name: '   ',
        agent_capability: 'hr-specialist',
      }),
    ).toBe('hr-specialist')
  })

  it('uses image basename (strip tag) when no name or capability', () => {
    expect(
      displayName({
        peer_id: peer,
        agent_image_ref: 'ghcr.io/yourorg/hr-agent:v1',
      }),
    ).toBe('hr-agent')
  })

  it('handles an image_ref without a tag', () => {
    expect(
      displayName({ peer_id: peer, agent_image_ref: 'local/notebook' }),
    ).toBe('notebook')
  })

  it('falls back to short peer id when nothing else is set', () => {
    expect(displayName({ peer_id: peer })).toMatch(/^12D3Ko…/)
  })

  it('handles totally empty input gracefully', () => {
    expect(displayName({ peer_id: '' })).toBe('(unknown agent)')
  })
})

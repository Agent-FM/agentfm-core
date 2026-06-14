import { describe, it, expect } from 'vitest'
import { API_CATALOG, ROUTE_MANIFEST, basePathOf } from '../../src/lib/apiCatalog'

describe('API_CATALOG', () => {
  it('has unique ids and non-empty paths/methods', () => {
    const ids = new Set<string>()
    for (const ep of API_CATALOG) {
      expect(ep.id).toBeTruthy()
      expect(ids.has(ep.id)).toBe(false)
      ids.add(ep.id)
      expect(ep.path.startsWith('/')).toBe(true)
      expect(['GET', 'POST']).toContain(ep.method)
      expect(['OpenAI-compatible', 'AgentFM-native', 'System']).toContain(ep.group)
      expect(['none', 'dispatch', 'signed']).toContain(ep.sideEffect)
    }
  })

  it('every param has a location and the body param (if any) carries an example object', () => {
    for (const ep of API_CATALOG) {
      for (const p of ep.params) {
        expect(['path', 'query', 'body']).toContain(p.loc)
        expect(p.name).toBeTruthy()
      }
      const bodies = ep.params.filter((p) => p.loc === 'body')
      expect(bodies.length).toBeLessThanOrEqual(1)
      if (bodies.length === 1) {
        expect(typeof bodies[0].example).toBe('object')
        expect(bodies[0].example).not.toBeNull()
      }
    }
  })

  it('drift guard: every catalog path maps onto a real Boss route prefix', () => {
    for (const ep of API_CATALOG) {
      const base = basePathOf(ep.path)
      const known = ROUTE_MANIFEST.some((r) => base === r || base.startsWith(r + '/'))
      expect(known, `${ep.path} (base ${base}) not in ROUTE_MANIFEST`).toBe(true)
    }
  })

  it('covers both OpenAI-compatible and AgentFM-native groups', () => {
    const groups = new Set(API_CATALOG.map((e) => e.group))
    expect(groups.has('OpenAI-compatible')).toBe(true)
    expect(groups.has('AgentFM-native')).toBe(true)
  })
})

import { describe, it, expect } from 'vitest'
import { resolvePath, buildRequest } from '../../src/lib/apiExplorer'
import type { EndpointDef } from '../../src/lib/apiCatalog'

const GET_EP: EndpointDef = {
  id: 'workers', group: 'AgentFM-native', method: 'GET', path: '/api/workers',
  summary: '', description: '',
  params: [{ name: 'include_offline', loc: 'query', required: false, example: 'true', description: '' }],
  exampleResponse: {}, sideEffect: 'none',
}

const POST_EP: EndpointDef = {
  id: 'self', group: 'AgentFM-native', method: 'POST', path: '/v1/peers/:peerId/comments/self',
  summary: '', description: '',
  params: [
    { name: 'peerId', loc: 'path', required: true, example: 'P', description: '' },
    { name: 'body', loc: 'body', required: true, example: { body: 'hi' }, description: '' },
  ],
  exampleResponse: {}, sideEffect: 'signed',
}

describe('resolvePath', () => {
  it('substitutes path params and url-encodes them', () => {
    expect(resolvePath('/v1/peers/:peerId', { peerId: 'a/b' })).toBe('/v1/peers/a%2Fb')
  })
})

describe('buildRequest', () => {
  it('appends query params only when present', () => {
    const { url, init } = buildRequest(GET_EP, { include_offline: 'true' }, 'http://h')
    expect(url).toBe('http://h/api/workers?include_offline=true')
    expect(init.method).toBe('GET')
    expect(init.body).toBeUndefined()
  })

  it('omits empty query params', () => {
    const { url } = buildRequest(GET_EP, { include_offline: '' }, 'http://h')
    expect(url).toBe('http://h/api/workers')
  })

  it('builds a JSON body and content-type for POST', () => {
    const { url, init } = buildRequest(POST_EP, { peerId: 'P', body: '{"body":"hi"}' }, 'http://h')
    expect(url).toBe('http://h/v1/peers/P/comments/self')
    expect(init.method).toBe('POST')
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"body":"hi"}')
  })

  it('adds an Authorization header when an api key is given', () => {
    const { init } = buildRequest(GET_EP, {}, 'http://h', 'secret')
    expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer secret')
  })
})

import { describe, it, expect } from 'vitest'
import { genCurl, genFetch, genPython, genOpenAI, SNIPPET_LANGS } from '../../src/lib/snippets'
import type { EndpointDef } from '../../src/lib/apiCatalog'

const CHAT: EndpointDef = {
  id: 'chat', group: 'OpenAI-compatible', method: 'POST', path: '/v1/chat/completions',
  summary: '', description: '',
  params: [{ name: 'body', loc: 'body', required: true, example: { model: 'llama3.2', messages: [] }, description: '' }],
  exampleResponse: {}, sideEffect: 'dispatch',
}

const WORKERS: EndpointDef = {
  id: 'workers', group: 'AgentFM-native', method: 'GET', path: '/api/workers',
  summary: '', description: '', params: [], exampleResponse: {}, sideEffect: 'none',
}

describe('snippets', () => {
  it('curl includes method, url and JSON body', () => {
    const s = genCurl(CHAT, { body: '{"model":"llama3.2"}' }, 'http://127.0.0.1:8080')
    expect(s).toContain('curl -X POST')
    expect(s).toContain('http://127.0.0.1:8080/v1/chat/completions')
    expect(s).toContain('{"model":"llama3.2"}')
  })

  it('fetch snippet references the resolved url', () => {
    const s = genFetch(WORKERS, {}, 'http://127.0.0.1:8080')
    expect(s).toContain("fetch('http://127.0.0.1:8080/api/workers'")
  })

  it('python snippet uses requests with the right method and url', () => {
    const s = genPython(WORKERS, {}, 'http://127.0.0.1:8080')
    expect(s).toContain('import requests')
    expect(s).toContain('requests.get(')
    expect(s).toContain("'http://127.0.0.1:8080/api/workers'")
  })

  it('python POST body is parsed via json.loads (valid Python for true/false/null)', () => {
    const s = genPython(CHAT, { body: '{"model":"llama3.2","stream":false}' }, 'http://127.0.0.1:8080')
    expect(s).toContain('import requests, json')
    expect(s).toContain(`json.loads('''{"model":"llama3.2","stream":false}''')`)
    // raw JSON must NOT be passed directly as a Python literal
    expect(s).not.toContain('json={"model"')
  })

  it('openai snippet is only meaningful for OpenAI-compatible endpoints', () => {
    const s = genOpenAI(CHAT, { body: '{"model":"llama3.2"}' }, 'http://127.0.0.1:8080')
    expect(s).toContain('from openai import OpenAI')
    expect(s).toContain('base_url="http://127.0.0.1:8080/v1"')
    expect(genOpenAI(WORKERS, {}, 'http://127.0.0.1:8080')).toBe('')
  })

  it('exposes the language list', () => {
    expect(SNIPPET_LANGS.map((l) => l.id)).toEqual(['curl', 'fetch', 'python', 'openai'])
  })
})

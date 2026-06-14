import type { EndpointDef, FormValues } from './apiCatalog'
import { buildRequest } from './apiExplorer'

export const SNIPPET_LANGS = [
  { id: 'curl', label: 'curl' },
  { id: 'fetch', label: 'JavaScript' },
  { id: 'python', label: 'Python' },
  { id: 'openai', label: 'OpenAI SDK' },
] as const

export type SnippetLang = (typeof SNIPPET_LANGS)[number]['id']

export function genCurl(ep: EndpointDef, values: FormValues, baseURL: string, apiKey?: string): string {
  const { url, init } = buildRequest(ep, values, baseURL, apiKey)
  const lines = [`curl -X ${ep.method} '${url}'`]
  if (apiKey) lines.push(`  -H 'Authorization: Bearer ${apiKey}'`)
  if (init.body) {
    lines.push(`  -H 'Content-Type: application/json'`)
    lines.push(`  -d '${init.body}'`)
  }
  return lines.join(' \\\n')
}

export function genFetch(ep: EndpointDef, values: FormValues, baseURL: string, apiKey?: string): string {
  const { url, init } = buildRequest(ep, values, baseURL, apiKey)
  const opts: Record<string, unknown> = { method: ep.method }
  if (init.headers && Object.keys(init.headers).length) opts.headers = init.headers
  if (init.body) opts.body = init.body as string
  return `const res = await fetch('${url}', ${JSON.stringify(opts, null, 2)})\nconsole.log(await res.text())`
}

export function genPython(ep: EndpointDef, values: FormValues, baseURL: string, apiKey?: string): string {
  const { url, init } = buildRequest(ep, values, baseURL, apiKey)
  const fn = ep.method === 'GET' ? 'get' : 'post'
  const args = [`'${url}'`]
  if (init.body) args.push(`json=${init.body}`)
  if (apiKey) args.push(`headers={'Authorization': 'Bearer ${apiKey}'}`)
  return `import requests\nres = requests.${fn}(${args.join(', ')})\nprint(res.text)`
}

export function genOpenAI(ep: EndpointDef, values: FormValues, baseURL: string, apiKey?: string): string {
  if (ep.group !== 'OpenAI-compatible') return ''
  const key = apiKey || 'not-needed'
  if (ep.id === 'models') {
    return `from openai import OpenAI\nclient = OpenAI(base_url="${baseURL}/v1", api_key="${key}")\nprint(client.models.list())`
  }
  const bodyStr = values.body ?? JSON.stringify(ep.params.find((p) => p.loc === 'body')?.example ?? {}, null, 2)
  let body: { model?: string; messages?: unknown; prompt?: unknown } = {}
  try { body = JSON.parse(bodyStr) } catch { /* leave defaults */ }
  if (ep.id === 'completions') {
    return `from openai import OpenAI\nclient = OpenAI(base_url="${baseURL}/v1", api_key="${key}")\nres = client.completions.create(model="${body.model ?? 'llama3.2'}", prompt=${JSON.stringify(body.prompt ?? 'Hello')})\nprint(res)`
  }
  return `from openai import OpenAI\nclient = OpenAI(base_url="${baseURL}/v1", api_key="${key}")\nres = client.chat.completions.create(\n    model="${body.model ?? 'llama3.2'}",\n    messages=${JSON.stringify(body.messages ?? [{ role: 'user', content: 'Hello' }])},\n)\nprint(res)`
}

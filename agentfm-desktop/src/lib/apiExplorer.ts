import type { EndpointDef, FormValues } from './apiCatalog'
import { getApiBaseURL } from './api'

export interface ExplorerResult {
  ok: boolean
  status: number
  ms: number
  body: string
  contentType: string
  error?: string
}

export function resolvePath(path: string, values: FormValues): string {
  return path.replace(/:([A-Za-z0-9_]+)/g, (_, name: string) =>
    encodeURIComponent(values[name] ?? ''),
  )
}

export function buildRequest(
  ep: EndpointDef,
  values: FormValues,
  baseURL: string,
  apiKey?: string,
): { url: string; init: RequestInit } {
  const path = resolvePath(ep.path, values)
  const query: string[] = []
  let bodyText: string | undefined

  for (const p of ep.params) {
    if (p.loc === 'query') {
      const v = values[p.name]
      if (v) query.push(`${encodeURIComponent(p.name)}=${encodeURIComponent(v)}`)
    } else if (p.loc === 'body') {
      bodyText = values[p.name] ?? JSON.stringify(p.example, null, 2)
    }
  }

  const headers: Record<string, string> = {}
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`

  const init: RequestInit = { method: ep.method, headers }
  if (bodyText !== undefined && ep.method !== 'GET') {
    headers['Content-Type'] = 'application/json'
    init.body = bodyText
  }

  const url = baseURL + path + (query.length ? `?${query.join('&')}` : '')
  return { url, init }
}

export async function sendRequest(
  ep: EndpointDef,
  values: FormValues,
  apiKey?: string,
): Promise<ExplorerResult> {
  const { url, init } = buildRequest(ep, values, getApiBaseURL(), apiKey)
  const start = performance.now()
  try {
    const res = await fetch(url, init)
    const body = await res.text()
    return {
      ok: res.ok,
      status: res.status,
      ms: Math.round(performance.now() - start),
      body,
      contentType: res.headers.get('content-type') ?? '',
    }
  } catch (e) {
    return {
      ok: false,
      status: 0,
      ms: Math.round(performance.now() - start),
      body: '',
      contentType: '',
      error: e instanceof Error ? e.message : String(e),
    }
  }
}

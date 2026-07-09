import { useState } from 'react'
import type { EndpointDef, FormValues } from '../../lib/apiCatalog'
import { SNIPPET_LANGS, genCurl, genFetch, genPython, genOpenAI } from '../../lib/snippets'
import type { SnippetLang } from '../../lib/snippets'

interface Props {
  endpoint: EndpointDef
  values: FormValues
  baseURL: string
  apiKey?: string
}

function gen(lang: SnippetLang, ep: EndpointDef, values: FormValues, baseURL: string, apiKey?: string): string {
  switch (lang) {
    case 'curl': return genCurl(ep, values, baseURL, apiKey)
    case 'fetch': return genFetch(ep, values, baseURL, apiKey)
    case 'python': return genPython(ep, values, baseURL, apiKey)
    case 'openai': return genOpenAI(ep, values, baseURL, apiKey)
  }
}

export function SnippetTabs({ endpoint, values, baseURL, apiKey }: Props) {
  const langs = SNIPPET_LANGS.filter((l) => l.id !== 'openai' || endpoint.group === 'OpenAI-compatible')
  const [lang, setLang] = useState<SnippetLang>('curl')
  const active = langs.some((l) => l.id === lang) ? lang : 'curl'
  const code = gen(active, endpoint, values, baseURL, apiKey)

  return (
    <div className="glass rounded-card p-3">
      <div className="flex gap-1 mb-2">
        {langs.map((l) => (
          <button
            key={l.id}
            onClick={() => setLang(l.id)}
            className={`px-3 py-1 rounded-ctl text-xs transition-colors ${
              l.id === active ? 'text-accent' : 'text-text-2 hover:text-text-0'
            }`}
          >
            {l.label}
          </button>
        ))}
        <button
          onClick={() => navigator.clipboard?.writeText(code)}
          className="ml-auto px-3 py-1 rounded-ctl text-xs text-text-2 hover:text-text-0"
        >
          Copy
        </button>
      </div>
      <pre className="glass-inset rounded-card p-3 font-mono text-xs overflow-auto whitespace-pre-wrap text-text-1">
        {code}
      </pre>
    </div>
  )
}

import type { ExplorerResult } from '../../lib/apiExplorer'

interface Props {
  result: ExplorerResult | null
  loading: boolean
}

function pretty(body: string, contentType: string): string {
  if (contentType.includes('json')) {
    try { return JSON.stringify(JSON.parse(body), null, 2) } catch { /* fall through */ }
  }
  return body
}

export function ResponsePanel({ result, loading }: Props) {
  if (loading) {
    return <div className="glass rounded-2xl p-4 text-sm text-text-2">Sending…</div>
  }
  if (!result) {
    return <div className="glass rounded-2xl p-4 text-sm text-text-2">Send a request to see the response.</div>
  }
  if (result.status === 0) {
    return (
      <div className="glass rounded-2xl p-4 text-sm">
        <span className="text-bad font-semibold">Backend not reachable.</span>
        <span className="text-text-2"> {result.error}</span>
      </div>
    )
  }
  const ok = result.ok
  return (
    <div className="glass rounded-2xl p-4">
      <div className="flex items-center gap-3 mb-2 text-xs">
        <span className={`font-mono font-semibold ${ok ? 'text-ok' : 'text-bad'}`}>{result.status}</span>
        <span className="text-text-2">{result.ms} ms</span>
        <span className="text-text-3 font-mono">{result.contentType}</span>
      </div>
      <pre className="bg-bg-0 border border-border-0 rounded-xl p-3 font-mono text-xs overflow-auto max-h-[360px] whitespace-pre-wrap text-text-1">
        {pretty(result.body, result.contentType)}
      </pre>
    </div>
  )
}

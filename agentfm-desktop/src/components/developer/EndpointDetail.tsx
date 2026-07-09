// src/components/developer/EndpointDetail.tsx
import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import type { EndpointDef, FormValues } from '../../lib/apiCatalog'
import { sendRequest } from '../../lib/apiExplorer'
import type { ExplorerResult } from '../../lib/apiExplorer'
import { getApiBaseURL } from '../../lib/api'
import { openEventStream } from '../../lib/sse'
import type { SseHandle } from '../../lib/sse'
import { RequestForm } from './RequestForm'
import { ResponsePanel } from './ResponsePanel'
import { SnippetTabs } from './SnippetTabs'
import { ConfirmDispatchDialog } from './ConfirmDispatchDialog'
import { StreamingView } from '../StreamingView'
import { Button } from '../primitives/Button'

interface Props {
  endpoint: EndpointDef
}

function MethodBadge({ method }: { method: 'GET' | 'POST' }) {
  return (
    <span
      className={`font-mono text-2xs font-semibold px-2 py-0.5 rounded-ctl ${
        method === 'GET' ? 'bg-ok/15 text-ok' : 'bg-accent/15 text-accent'
      }`}
    >
      {method}
    </span>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h3 className="text-2xs font-medium text-text-2 mb-2">
        {title}
      </h3>
      {children}
    </section>
  )
}

function Code({ children }: { children: string }) {
  return (
    <pre className="console-well mono-console p-3 text-xs overflow-auto whitespace-pre-wrap">
      {children}
    </pre>
  )
}

export function EndpointDetail({ endpoint }: Props) {
  const [values, setValues] = useState<FormValues>({})
  const [result, setResult] = useState<ExplorerResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [stream, setStream] = useState('')
  const [streaming, setStreaming] = useState(false)
  const sseRef = useRef<SseHandle | null>(null)
  const reqToken = useRef(0)

  useEffect(() => () => { sseRef.current?.close() }, [])

  const isSse = endpoint.streaming === 'sse'

  // Reset transient state when the selected endpoint changes.
  const [lastId, setLastId] = useState(endpoint.id)
  if (lastId !== endpoint.id) {
    setLastId(endpoint.id)
    reqToken.current++
    setValues({})
    setResult(null)
    setStream('')
    setStreaming(false)
    // A request in flight when the endpoint changes returns after the token
    // bump above and bails without clearing loading, reset it here so the
    // Send button doesn't stick on "Sending…" forever.
    setLoading(false)
    sseRef.current?.close()
    sseRef.current = null
  }

  function startSse() {
    setStream('')
    setStreaming(true)
    setResult(null)
    sseRef.current = openEventStream(getApiBaseURL() + endpoint.path, {
      onEvent: (type, data) =>
        setStream((s) => `${s}event: ${type}\n${data ? JSON.stringify(data) : ''}\n\n`),
    })
  }

  function stopSse() {
    sseRef.current?.close()
    sseRef.current = null
    setStreaming(false)
  }

  async function fire() {
    const myToken = ++reqToken.current
    setLoading(true)
    setConfirming(false)
    const r = await sendRequest(endpoint, values)
    if (myToken !== reqToken.current) return
    setResult(r)
    setLoading(false)
  }

  function onSend() {
    if (isSse) {
      if (streaming) stopSse()
      else startSse()
    } else if (endpoint.sideEffect === 'none') {
      void fire()
    } else {
      setConfirming(true)
    }
  }

  const bodyParam = endpoint.params.find((p) => p.loc === 'body')
  const scalarParams = endpoint.params.filter((p) => p.loc !== 'body')
  const exampleText =
    typeof endpoint.exampleResponse === 'string'
      ? endpoint.exampleResponse
      : JSON.stringify(endpoint.exampleResponse, null, 2)

  return (
    <div className="space-y-5 min-w-0">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2 font-mono text-xs mb-1.5 min-w-0">
          <MethodBadge method={endpoint.method} />
          <span className="text-text-0 break-all">{endpoint.path}</span>
        </div>
        <h2 className="text-lg font-semibold text-text-0">{endpoint.summary}</h2>
        <p className="text-sm text-text-1 mt-1">{endpoint.description}</p>
      </div>

      {endpoint.overview && (
        <Section title="Overview">
          <p className="text-sm text-text-1 leading-relaxed">{endpoint.overview}</p>
        </Section>
      )}

      {endpoint.whenToUse && (
        <Section title="When to use">
          <p className="text-sm text-text-1 leading-relaxed">{endpoint.whenToUse}</p>
        </Section>
      )}

      {endpoint.auth && (
        <Section title="Authentication">
          <p className="text-sm text-text-1 leading-relaxed">{endpoint.auth}</p>
        </Section>
      )}

      {scalarParams.length > 0 && (
        <Section title="Parameters">
          <div className="overflow-hidden border border-border-0">
            <table className="w-full text-left text-xs">
              <thead className="bg-chrome text-text-1">
                <tr>
                  <th className="font-mono font-medium px-3 py-1">Name</th>
                  <th className="font-mono font-medium px-3 py-1">In</th>
                  <th className="font-mono font-medium px-3 py-1">Required</th>
                  <th className="font-mono font-medium px-3 py-1">Description</th>
                </tr>
              </thead>
              <tbody>
                {scalarParams.map((p) => (
                  <tr key={p.name} className="border-t border-border-0 align-top">
                    <td className="font-mono text-accent px-3 py-1.5">{p.name}</td>
                    <td className="font-mono text-text-2 px-3 py-1.5">{p.loc}</td>
                    <td className="px-3 py-1.5">{p.required ? <span className="text-bad">yes</span> : <span className="text-text-2">no</span>}</td>
                    <td className="text-text-1 px-3 py-1.5">
                      {p.description}
                      {p.example !== undefined && p.example !== '' && (
                        <span className="block text-text-2 font-mono text-2xs mt-0.5">e.g. {String(p.example)}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      )}

      {bodyParam && (
        <Section title="Request body">
          <p className="text-xs text-text-2 mb-2">{bodyParam.description}</p>
          <Code>{JSON.stringify(bodyParam.example, null, 2)}</Code>
        </Section>
      )}

      <Section title="Response">
        {endpoint.responseFields && endpoint.responseFields.length > 0 && (
          <div className="overflow-hidden border border-border-0 mb-3">
            <table className="w-full text-left text-xs">
              <thead className="bg-chrome text-text-1">
                <tr>
                  <th className="font-mono font-medium px-3 py-1">Field</th>
                  <th className="font-mono font-medium px-3 py-1">Type</th>
                  <th className="font-mono font-medium px-3 py-1">Description</th>
                </tr>
              </thead>
              <tbody>
                {endpoint.responseFields.map((f) => (
                  <tr key={f.name} className="border-t border-border-0 align-top">
                    <td className="font-mono text-accent px-3 py-1.5 whitespace-nowrap">{f.name}</td>
                    <td className="font-mono text-text-2 px-3 py-1.5 whitespace-nowrap">{f.type}</td>
                    <td className="text-text-1 px-3 py-1.5">{f.description}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="text-2xs font-medium text-text-2 mb-1.5">Example</div>
        <Code>{exampleText}</Code>
      </Section>

      {endpoint.errors && endpoint.errors.length > 0 && (
        <Section title="Errors">
          <div className="overflow-hidden border border-border-0">
            <table className="w-full text-left text-xs">
              <thead className="bg-chrome text-text-1">
                <tr>
                  <th className="font-mono font-medium px-3 py-1">Status</th>
                  <th className="font-mono font-medium px-3 py-1">When</th>
                </tr>
              </thead>
              <tbody>
                {endpoint.errors.map((e) => (
                  <tr key={e.status} className="border-t border-border-0 align-top">
                    <td className="font-mono text-bad tabular-nums px-3 py-1.5 whitespace-nowrap">{e.status}</td>
                    <td className="text-text-1 px-3 py-1.5">{e.when}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      )}

      {endpoint.notes && (
        <div className="rounded-card border border-accent/25 bg-accent/[0.06] px-3.5 py-3 text-xs text-text-1 leading-relaxed">
          <span className="font-semibold text-accent">Note </span>
          {endpoint.notes}
        </div>
      )}

      {/* Try it */}
      <Section title="Try it">
        <div className="space-y-3">
          <RequestForm endpoint={endpoint} values={values} onChange={setValues} />
          <div>
            {isSse ? (
              <Button onClick={onSend}>{streaming ? 'Stop stream' : 'Start stream'}</Button>
            ) : (
              <Button variant="primary" onClick={onSend} disabled={loading}>
                {loading ? 'Sending…' : 'Send'}
              </Button>
            )}
          </div>
          {isSse ? (
            <StreamingView output={stream || 'Start the stream to see live events.'} streaming={streaming} />
          ) : (
            <ResponsePanel result={result} loading={loading} />
          )}
        </div>
      </Section>

      {/* Code */}
      <Section title="Code">
        <SnippetTabs endpoint={endpoint} values={values} baseURL={getApiBaseURL()} />
      </Section>

      {confirming && endpoint.sideEffect !== 'none' && (
        <ConfirmDispatchDialog
          sideEffect={endpoint.sideEffect}
          onConfirm={() => void fire()}
          onCancel={() => setConfirming(false)}
        />
      )}
    </div>
  )
}

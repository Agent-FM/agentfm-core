import { useState } from 'react'
import type { EndpointDef, FormValues } from '../../lib/apiCatalog'
import { sendRequest } from '../../lib/apiExplorer'
import type { ExplorerResult } from '../../lib/apiExplorer'
import { getApiBaseURL } from '../../lib/api'
import { RequestForm } from './RequestForm'
import { ResponsePanel } from './ResponsePanel'
import { SnippetTabs } from './SnippetTabs'
import { ConfirmDispatchDialog } from './ConfirmDispatchDialog'
import { GradientButton } from '../primitives/GradientButton'

interface Props {
  endpoint: EndpointDef
}

export function EndpointDetail({ endpoint }: Props) {
  const [values, setValues] = useState<FormValues>({})
  const [result, setResult] = useState<ExplorerResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [confirming, setConfirming] = useState(false)

  // Reset transient state when the selected endpoint changes.
  const [lastId, setLastId] = useState(endpoint.id)
  if (lastId !== endpoint.id) {
    setLastId(endpoint.id)
    setValues({})
    setResult(null)
  }

  async function fire() {
    setLoading(true)
    setConfirming(false)
    const r = await sendRequest(endpoint, values)
    setResult(r)
    setLoading(false)
  }

  function onSend() {
    if (endpoint.sideEffect === 'none') void fire()
    else setConfirming(true)
  }

  return (
    <div className="flex-1 min-w-0 space-y-4">
      <div>
        <div className="font-mono text-sm mb-1">
          <span className="text-accent">{endpoint.method}</span> {endpoint.path}
        </div>
        <p className="text-sm text-text-1">{endpoint.description}</p>
      </div>

      <RequestForm endpoint={endpoint} values={values} onChange={setValues} />

      <div>
        <GradientButton onClick={onSend} disabled={loading}>
          {loading ? 'Sending…' : 'Send'}
        </GradientButton>
      </div>

      <ResponsePanel result={result} loading={loading} />
      <SnippetTabs endpoint={endpoint} values={values} baseURL={getApiBaseURL()} />

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

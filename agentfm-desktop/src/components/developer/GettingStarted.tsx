import { Card } from '../primitives/Card'

interface Props {
  baseURL: string
  authEnabled: boolean
}

export function GettingStarted({ baseURL, authEnabled }: Props) {
  const sdk = `from openai import OpenAI\nclient = OpenAI(base_url="${baseURL}/v1", api_key="not-needed")\nprint(client.models.list())`
  return (
    <Card density="spacious" className="mb-5">
      <h2 className="text-lg font-semibold mb-2">Getting started</h2>
      <p className="text-sm text-text-1 mb-3">
        The Boss API is live at <span className="font-mono text-accent">{baseURL}</span> whenever this app is open.
        {' '}
        {authEnabled
          ? 'Auth is enabled — send an Authorization: Bearer header.'
          : 'No API key needed (loopback, auth disabled by default).'}
      </p>
      <pre className="bg-bg-0 border border-border-0 rounded-xl p-3 font-mono text-xs overflow-auto whitespace-pre-wrap text-text-1">
        {sdk}
      </pre>
    </Card>
  )
}

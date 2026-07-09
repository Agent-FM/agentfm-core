import { Card } from '../primitives/Card'

interface Props {
  baseURL: string
  authEnabled: boolean
}

interface Step {
  n: number
  title: string
  body: string
  code: string
}

export function GettingStarted({ baseURL, authEnabled }: Props) {
  const authNote = authEnabled
    ? 'Auth is enabled, send an Authorization: Bearer <key> header on every request.'
    : 'No API key needed, the gateway binds to loopback and auth is disabled by default.'

  const steps: Step[] = [
    {
      n: 1,
      title: 'Check the gateway is up',
      body: 'A 200 from /health means the Boss is online and ready to route work to the mesh.',
      code: `curl ${baseURL}/health`,
    },
    {
      n: 2,
      title: 'List the agents (models) online',
      body: 'OpenAI-compatible. Each online worker is exposed as a model whose id is the agent’s peer id, that is the handle you use to address it.',
      code: `curl ${baseURL}/v1/models`,
    },
    {
      n: 3,
      title: 'Dispatch a task to an agent',
      body: 'POST a prompt to /api/execute, addressing the agent by its peer id in worker_id (from step 2). The Boss runs it in a sandbox and returns the result. Use /api/execute/async for fire-and-forget (returns 202).',
      code: `curl -X POST ${baseURL}/api/execute \\
  -H 'Content-Type: application/json' \\
  -d '{"prompt": "sick leave", "worker_id": "<AGENT_PEER_ID>"}'`,
    },
    {
      n: 4,
      title: 'Stream live mesh events',
      body: 'Server-Sent Events: telemetry, task progress, and artifact notifications as they happen.',
      code: `curl -N ${baseURL}/v1/events`,
    },
  ]

  const python = `from openai import OpenAI

# The Boss speaks the OpenAI protocol, point any OpenAI client at it.
client = OpenAI(base_url="${baseURL}/v1", api_key="${authEnabled ? 'YOUR_KEY' : 'not-needed'}")

# 1. Discover online agents, each model id IS the agent's peer id
agent_id = client.models.list().data[0].id
print("addressing agent:", agent_id)

# 2. Run a task and stream the agent's output (address it by peer id)
stream = client.chat.completions.create(
    model=agent_id,
    messages=[{"role": "user", "content": "sick leave"}],
    stream=True,
)
for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="", flush=True)`

  return (
    <Card density="spacious" className="mb-5">
      <h2 className="text-lg font-semibold mb-1.5">Getting started</h2>
      <p className="text-sm text-text-1 mb-1">
        The Boss API is live at{' '}
        <span className="font-mono text-accent">{baseURL}</span> whenever this app is open. It exposes
        both an <span className="text-text-0">OpenAI-compatible</span> surface (
        <span className="font-mono text-2xs">/v1/*</span>) and{' '}
        <span className="text-text-0">AgentFM-native</span> endpoints (
        <span className="font-mono text-2xs">/api/*</span>) that dispatch work to live workers on your mesh.
      </p>
      <p className="text-2xs text-text-2 mb-4">{authNote}</p>

      <div className="space-y-3 mb-4">
        {steps.map((s) => (
          <div key={s.n} className="flex gap-3">
            <span className="shrink-0 mt-0.5 flex items-center justify-center w-5 h-5 rounded-full bg-accent/15 text-accent font-mono text-xs font-bold">
              {s.n}
            </span>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-text-0">{s.title}</div>
              <div className="text-2xs text-text-2 mb-1.5">{s.body}</div>
              <pre className="glass-inset rounded-ctl p-2.5 font-mono text-2xs overflow-auto whitespace-pre-wrap text-text-1">
                {s.code}
              </pre>
            </div>
          </div>
        ))}
      </div>

      <div className="text-2xs font-medium text-text-2 mb-1.5">
        Or use the OpenAI Python SDK
      </div>
      <pre className="glass-inset rounded-card p-3 font-mono text-xs overflow-auto whitespace-pre-wrap text-text-1">
        {python}
      </pre>
      <p className="text-2xs text-text-2 mt-3">
        Full request/response shapes, parameters, and copy-paste snippets for every endpoint are in the
        explorer below.
      </p>
    </Card>
  )
}

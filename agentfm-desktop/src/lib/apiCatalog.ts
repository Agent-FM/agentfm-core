export type SideEffect = 'none' | 'dispatch' | 'signed'
export type ParamLoc = 'path' | 'query' | 'body'
export type EndpointGroup = 'OpenAI-compatible' | 'AgentFM-native' | 'System'

export interface ParamDef {
  name: string
  loc: ParamLoc
  required: boolean
  example: unknown
  description: string
}

export interface EndpointDef {
  id: string
  group: EndpointGroup
  method: 'GET' | 'POST'
  path: string
  summary: string
  description: string
  params: ParamDef[]
  exampleResponse: unknown
  streaming?: 'sse' | 'tokens'
  sideEffect: SideEffect
}

// Mux registrations in internal/boss/api.go. Drift guard checks each
// catalog path resolves onto one of these prefixes.
export const ROUTE_MANIFEST = [
  '/v1/about',
  '/v1/events',
  '/v1/models',
  '/v1/chat/completions',
  '/v1/completions',
  '/v1/peers',
  '/api/workers',
  '/api/execute',
  '/api/execute/async',
  '/api/relay/test',
  '/health',
  '/metrics',
] as const

// Strip trailing :params and querystring to compare against ROUTE_MANIFEST.
export function basePathOf(path: string): string {
  const noQuery = path.split('?')[0]
  const segs = noQuery.split('/').filter(Boolean)
  const kept: string[] = []
  for (const s of segs) {
    if (s.startsWith(':')) break
    kept.push(s)
  }
  return '/' + kept.join('/')
}

const EXECUTE_BODY = {
  prompt: 'Summarize our sick-leave policy in 3 bullets.',
  worker_id: '',
}

const CHAT_BODY = {
  model: 'llama3.2',
  messages: [{ role: 'user', content: 'Hello from the API explorer' }],
  stream: false,
}

export const API_CATALOG: EndpointDef[] = [
  {
    id: 'health',
    group: 'System',
    method: 'GET',
    path: '/health',
    summary: 'Liveness + online worker count',
    description: 'Returns gateway status and how many workers are currently online on the mesh.',
    params: [],
    exampleResponse: { status: 'ok', online_workers: 1 },
    sideEffect: 'none',
  },
  {
    id: 'about',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/about',
    summary: 'Node + relay identity',
    description: 'Boss peer id, relay peer id, version, connection mode.',
    params: [],
    exampleResponse: { version: '1.3.0', relay_peer_id: '12D3Koo…' },
    sideEffect: 'none',
  },
  {
    id: 'workers',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/api/workers',
    summary: 'List workers',
    description: 'Snapshot of workers seen on telemetry. Set include_offline to also list remembered offline peers.',
    params: [
      { name: 'include_offline', loc: 'query', required: false, example: 'true', description: 'Include remembered offline peers.' },
    ],
    exampleResponse: { workers: [{ peer_id: '12D3Koo…', agent: 'HR Agent', online: true }] },
    sideEffect: 'none',
  },
  {
    id: 'models',
    group: 'OpenAI-compatible',
    method: 'GET',
    path: '/v1/models',
    summary: 'List advertised models (OpenAI-compatible)',
    description: 'Models advertised by online workers, in OpenAI /v1/models shape.',
    params: [],
    exampleResponse: { object: 'list', data: [{ id: 'llama3.2', object: 'model' }] },
    sideEffect: 'none',
  },
  {
    id: 'events',
    group: 'System',
    method: 'GET',
    path: '/v1/events',
    summary: 'Live mesh events (SSE)',
    description: 'Server-Sent Events stream: worker_online, worker_offline, entry_appended, equivocator_marked.',
    params: [],
    exampleResponse: 'event: worker_online\ndata: {"peer_id":"12D3Koo…"}',
    streaming: 'sse',
    sideEffect: 'none',
  },
  {
    id: 'relay-test',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/api/relay/test',
    summary: 'Probe a relay multiaddr',
    description: 'Dials a candidate relay multiaddr and reports reachability. No mesh side effects.',
    params: [
      { name: 'body', loc: 'body', required: true, example: { multiaddr: '/ip4/127.0.0.1/tcp/4015/p2p/12D3Koo…' }, description: 'Relay multiaddr to probe.' },
    ],
    exampleResponse: { reachable: true, rtt_ms: 12 },
    sideEffect: 'none',
  },
  {
    id: 'execute',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/api/execute',
    summary: 'Dispatch a task (sync)',
    description: 'Dispatches a prompt to a worker and streams stdout back. Runs a real container on the worker.',
    params: [
      { name: 'body', loc: 'body', required: true, example: EXECUTE_BODY, description: 'prompt (required); worker_id optional — empty picks an available worker.' },
    ],
    exampleResponse: '[AGENTFM] task complete',
    streaming: 'tokens',
    sideEffect: 'dispatch',
  },
  {
    id: 'execute-async',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/api/execute/async',
    summary: 'Dispatch a task (async, 202)',
    description: 'Accepts the task and returns 202 immediately; work runs in the background. Runs a real container.',
    params: [
      { name: 'body', loc: 'body', required: true, example: EXECUTE_BODY, description: 'Same body as /api/execute.' },
    ],
    exampleResponse: { task_id: 'task_abc123', status: 'accepted' },
    sideEffect: 'dispatch',
  },
  {
    id: 'chat-completions',
    group: 'OpenAI-compatible',
    method: 'POST',
    path: '/v1/chat/completions',
    summary: 'Chat completion (OpenAI-compatible)',
    description: 'OpenAI chat-completions shape. Routes to a worker by model. Runs a real container.',
    params: [
      { name: 'body', loc: 'body', required: true, example: CHAT_BODY, description: 'model + messages; set stream:true for token streaming.' },
    ],
    exampleResponse: { id: 'chatcmpl-1', choices: [{ message: { role: 'assistant', content: '…' } }] },
    streaming: 'tokens',
    sideEffect: 'dispatch',
  },
  {
    id: 'completions',
    group: 'OpenAI-compatible',
    method: 'POST',
    path: '/v1/completions',
    summary: 'Text completion (OpenAI-compatible)',
    description: 'OpenAI completions shape. Routes to a worker by model. Runs a real container.',
    params: [
      { name: 'body', loc: 'body', required: true, example: { model: 'llama3.2', prompt: 'Hello' }, description: 'model + prompt.' },
    ],
    exampleResponse: { id: 'cmpl-1', choices: [{ text: '…' }] },
    streaming: 'tokens',
    sideEffect: 'dispatch',
  },
  {
    id: 'peer-summary',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId',
    summary: 'Peer reputation summary',
    description: 'Trust score, counts and metadata for one peer.',
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Target peer id.' },
    ],
    exampleResponse: { peer_id: '12D3KooW…', score: 0.42 },
    sideEffect: 'none',
  },
  {
    id: 'peer-self-comment',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/v1/peers/:peerId/comments/self',
    summary: 'Submit signed feedback',
    description: 'Submits a comment/feedback about a peer. The Boss signs it with its libp2p identity and appends to the ledger.',
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Peer the feedback is about.' },
      { name: 'body', loc: 'body', required: true, example: { body: 'Reliable worker.', rating: 0.3 }, description: 'Feedback text and rating.' },
    ],
    exampleResponse: { cid: 'bafy…', ledger_hash: 'sha256:…' },
    sideEffect: 'signed',
  },
]

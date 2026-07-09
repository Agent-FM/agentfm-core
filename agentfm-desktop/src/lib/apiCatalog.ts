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

export interface ResponseField {
  name: string
  type: string
  description: string
}

export interface ErrorDef {
  status: string
  when: string
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
  // Extended documentation (all optional; rendered only when present).
  overview?: string
  whenToUse?: string
  auth?: string
  responseFields?: ResponseField[]
  errors?: ErrorDef[]
  notes?: string
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

// Strip the querystring and everything from the first :param onward, then
// compare against ROUTE_MANIFEST. Note: static suffixes after a :param
// (e.g. the "comments/self" in /v1/peers/:peerId/comments/self) are
// intentionally not validated by the drift guard.
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

const AUTH_LOOPBACK =
  'The gateway binds to loopback (127.0.0.1) and auth is disabled by default, so no API key is needed. If you enable auth, send an Authorization: Bearer <key> header on every request.'
const AUTH_PUBLIC = 'Unauthenticated, served without auth even when the gateway has auth enabled.'

// Agents are addressed by their libp2p peer id (from /v1/models), never by a
// display name, names are not unique across a federated mesh.
const EXAMPLE_PEER = '12D3KooWNYnSuMbZPJwi94JP3m8qu756E9ygU9GdtwoLXS6QwZ9a'

const EXECUTE_BODY = {
  prompt: 'Summarize our sick-leave policy in 3 bullets.',
  worker_id: EXAMPLE_PEER,
}

const CHAT_BODY = {
  model: EXAMPLE_PEER,
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
    overview:
      'The simplest liveness probe. A 200 response means the Boss gateway is up and connected to the mesh. The body reports how many workers are currently advertising themselves on telemetry, so you can poll this to know when at least one agent is reachable before dispatching work.',
    whenToUse:
      'Poll on startup to wait for the mesh to be ready, or as a container/orchestrator health check.',
    auth: AUTH_PUBLIC,
    params: [],
    exampleResponse: { status: 'ok', online_workers: 1 },
    responseFields: [
      { name: 'status', type: 'string', description: '"ok" while the gateway is healthy.' },
      { name: 'online_workers', type: 'number', description: 'Count of workers seen online on telemetry right now.' },
    ],
    errors: [{ status: '503', when: 'Gateway is starting up or shutting down and not ready to serve.' }],
    sideEffect: 'none',
  },
  {
    id: 'about',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/about',
    summary: 'Node + relay identity',
    description: 'Boss peer id, relay peer id, version, connection mode.',
    overview:
      'Identity and build metadata for this Boss node: its libp2p peer id, the relay it is connected through, the backend version, and how long it has been running. Useful for diagnostics and for showing "which node am I talking to" in tooling.',
    whenToUse:
      'Display node identity in a dashboard, verify which relay/swarm you are connected to, or check the backend version before relying on a newer endpoint.',
    auth: AUTH_LOOPBACK,
    params: [],
    exampleResponse: {
      version: '1.3.0-rc.1',
      boss_peer_id: '12D3KooW…',
      relay_peer_id: '12D3KooW…',
      relay_multiaddr: '/ip4/127.0.0.1/tcp/4015/p2p/12D3KooW…',
      uptime_seconds: 9213,
    },
    responseFields: [
      { name: 'version', type: 'string', description: 'Backend build version (e.g. 1.3.0-rc.1).' },
      { name: 'boss_peer_id', type: 'string', description: 'This Boss node’s libp2p peer id.' },
      { name: 'relay_peer_id', type: 'string', description: 'Peer id of the relay/lighthouse it dials, or empty if not connected.' },
      { name: 'relay_multiaddr', type: 'string', description: 'Full multiaddr of the connected relay.' },
      { name: 'uptime_seconds', type: 'number', description: 'Seconds since this backend started.' },
    ],
    sideEffect: 'none',
  },
  {
    id: 'workers',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/api/workers',
    summary: 'List workers',
    description: 'Snapshot of workers seen on telemetry. Set include_offline to also list remembered offline peers.',
    overview:
      'Returns the current radar: every worker the Boss has seen broadcast a telemetry snapshot, with its agent name, capabilities, queue depth, CPU/GPU load and honesty score. By default only online workers are returned; pass include_offline=true to also list peers remembered from earlier sessions.',
    whenToUse:
      'Pick a target worker before dispatching, render a mesh overview, or check a specific agent’s load and reputation.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'include_offline', loc: 'query', required: false, example: 'true', description: 'Include remembered offline peers in the list.' },
    ],
    exampleResponse: {
      success: true,
      online_count: 1,
      offline_count: 0,
      agents: [
        { peer_id: '12D3KooW…', name: 'HR Agent', online: true, current_tasks: 0, max_tasks: 3, cpu_usage_pct: 24, honesty_score: 0.42 },
      ],
    },
    responseFields: [
      { name: 'agents[]', type: 'array', description: 'One entry per known worker.' },
      { name: 'agents[].peer_id', type: 'string', description: 'Worker libp2p peer id (use as worker_id when dispatching).' },
      { name: 'agents[].name', type: 'string', description: 'Human-readable agent name advertised by the worker.' },
      { name: 'agents[].online', type: 'boolean', description: 'Whether the worker is currently broadcasting telemetry.' },
      { name: 'agents[].current_tasks / max_tasks', type: 'number', description: 'Live queue depth vs the worker’s concurrency cap.' },
      { name: 'agents[].honesty_score', type: 'number', description: 'EigenTrust-style reputation in [-1, 1]; dispatch is refused below the project’s floor.' },
      { name: 'online_count / offline_count', type: 'number', description: 'Totals for the returned list.' },
    ],
    sideEffect: 'none',
  },
  {
    id: 'models',
    group: 'OpenAI-compatible',
    method: 'GET',
    path: '/v1/models',
    summary: 'List advertised models (OpenAI-compatible)',
    description: 'Models advertised by online workers, in OpenAI /v1/models shape.',
    overview:
      'The OpenAI-compatible model list. Each online worker is surfaced as a "model" whose id is the worker’s libp2p peer id, the stable, unique handle you pass as `model` when calling /v1/chat/completions. Agent display names are not listed because they are not unique across a federated mesh.',
    whenToUse:
      'Point an existing OpenAI SDK at the Boss, enumerate the agent peer ids, and use one as the `model` for a chat call.',
    auth: AUTH_LOOPBACK,
    params: [],
    exampleResponse: { object: 'list', data: [{ id: EXAMPLE_PEER, object: 'model', owned_by: 'agentfm' }] },
    responseFields: [
      { name: 'object', type: 'string', description: 'Always "list".' },
      { name: 'data[].id', type: 'string', description: 'The agent’s peer id, pass this as "model" to address it.' },
      { name: 'data[].object', type: 'string', description: 'Always "model".' },
    ],
    sideEffect: 'none',
  },
  {
    id: 'events',
    group: 'System',
    method: 'GET',
    path: '/v1/events',
    summary: 'Live mesh events (SSE)',
    description: 'Server-Sent Events stream: worker_online, worker_offline, entry_appended, equivocator_marked.',
    overview:
      'A long-lived Server-Sent Events stream of everything happening on the mesh: workers coming online or going offline, ledger entries being appended (ratings/comments), and equivocation alerts. Each message is an SSE `event:` line plus a JSON `data:` payload. The connection stays open until the client disconnects.',
    whenToUse:
      'Drive a live dashboard/radar without polling, or react to task and reputation changes as they happen.',
    auth: AUTH_LOOPBACK,
    params: [],
    exampleResponse: 'event: worker_online\ndata: {"peer_id":"12D3KooW…","agent":"HR Agent"}\n\n',
    responseFields: [
      { name: 'worker_online / worker_offline', type: 'event', description: 'Emitted when a worker starts/stops broadcasting telemetry.' },
      { name: 'entry_appended', type: 'event', description: 'A signed rating or comment was appended to the ledger.' },
      { name: 'equivocator_marked', type: 'event', description: 'A peer was flagged for double-signing (equivocation).' },
    ],
    notes: 'Use an SSE/EventSource client. Reconnect on disconnect; the stream does not replay missed events.',
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
    overview:
      'Dials a candidate relay/bootstrap multiaddr from this machine and reports whether it is reachable and which peer id answered. Read-only: it opens a transient connection and closes it. Handy when configuring a private swarm before committing the relay to a project.',
    whenToUse:
      'Validate a private relay address in a setup wizard, or debug why a node can’t join a swarm.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'body', loc: 'body', required: true, example: { multiaddr: '/ip4/127.0.0.1/tcp/4015/p2p/12D3KooW…' }, description: 'Relay multiaddr to probe.' },
    ],
    exampleResponse: { ok: true, peer_id: '12D3KooW…' },
    responseFields: [
      { name: 'ok', type: 'boolean', description: 'True if the relay was dialled successfully.' },
      { name: 'peer_id', type: 'string', description: 'Peer id that answered the dial (on success).' },
      { name: 'error', type: 'string', description: 'Failure reason when ok is false (omitted on success).' },
    ],
    errors: [
      { status: '400', when: 'The multiaddr is missing or malformed.' },
      { status: '200 ok:false', when: 'The address parsed but the relay was unreachable (timeout/refused).' },
    ],
    sideEffect: 'none',
  },
  {
    id: 'execute',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/api/execute',
    summary: 'Dispatch a task (sync)',
    description: 'Dispatches a prompt to a worker and streams stdout back. Runs a real container on the worker.',
    overview:
      'The core dispatch call. The Boss selects an eligible worker (or the one you name), opens a libp2p task stream, and the worker runs your prompt inside an ephemeral Podman container. The worker’s stdout is streamed straight back in the response as it is produced, ending with a completion marker. If the agent produces files, an artifact zip is harvested separately into ./agentfm_artifacts/.',
    whenToUse:
      'Run an agent and wait for its output inline. For fire-and-forget or webhook delivery, use /api/execute/async instead.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'body', loc: 'body', required: true, example: EXECUTE_BODY, description: 'prompt (required) and worker_id (required), the libp2p peer id of the target worker. This endpoint does not auto-pick; use /v1/chat/completions for model-based selection.' },
    ],
    exampleResponse: '…agent stdout streamed live…\n[AGENTFM: NO_FILES]\n[AGENTFM] task complete',
    responseFields: [
      { name: '(stream)', type: 'text', description: 'Raw worker stdout, forwarded line by line as the container runs.' },
      { name: '[AGENTFM: FILES_INCOMING]', type: 'marker', description: 'An artifact zip will follow on the artifact protocol.' },
      { name: '[AGENTFM: NO_FILES]', type: 'marker', description: 'The task produced no artifacts.' },
    ],
    errors: [
      { status: '400', when: 'prompt is missing/empty, task_id is malformed, or the body is not valid JSON.' },
      { status: '404', when: 'worker_id is missing, empty, or not online (returned as "Worker not found or offline").' },
    ],
    notes: 'Containers run with --network host on the worker; treat agent images as trusted code.',
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
    overview:
      'Same dispatch as /api/execute, but it returns 202 + a task id immediately and runs the work in the background. Optionally include webhook_url and the Boss will POST you the result when the task finishes (after waiting up to ~10s for any artifact zip to land). Omit it and the task still runs and writes artifacts to disk, just with no notification.',
    whenToUse:
      'Long-running agents, batch dispatch, or any caller that should not hold a connection open and prefers a callback over polling.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'body', loc: 'body', required: true, example: EXECUTE_BODY, description: 'Required: prompt and worker_id (the target agent’s peer id). OPTIONAL: add "webhook_url": "https://your-server/callback" to get a completion callback. The task dispatches and runs either way, webhook_url only controls whether you are notified. Field name is webhook_url (not "webhook").' },
    ],
    exampleResponse: { task_id: 'task_abc123', status: 'queued', message: 'Task dispatched to P2P mesh.' },
    responseFields: [
      { name: 'task_id', type: 'string', description: 'Server-generated id for the accepted task.' },
      { name: 'status', type: 'string', description: 'Always "queued" on the 202.' },
      { name: 'message', type: 'string', description: 'Human-readable dispatch note.' },
      { name: '(webhook) → your endpoint', type: 'callback', description: 'On completion the Boss POSTs JSON { task_id, worker_id, status: "completed" } to webhook_url, Content-Type application/json, with a signature header (verify it came from the Boss), 30s timeout.' },
    ],
    errors: [
      { status: '400', when: 'Invalid body / missing prompt.' },
      { status: '404', when: 'worker_id is missing, empty, or not online.' },
    ],
    notes: 'webhook_url MUST be publicly reachable, an SSRF guard refuses private/loopback IPs at dial time, so 127.0.0.1 / LAN addresses are rejected even though the API itself is loopback. For local testing, tunnel a public URL (e.g. ngrok) or use a receiver like webhook.site. Contract: the background worker is spawned BEFORE the 202 ack is written, so a 202 means the task is committed even if the ack write later fails. taskId is validated against SafeTaskIDPattern before any filesystem use.',
    sideEffect: 'dispatch',
  },
  {
    id: 'chat-completions',
    group: 'OpenAI-compatible',
    method: 'POST',
    path: '/v1/chat/completions',
    summary: 'Chat completion (OpenAI-compatible)',
    description: 'OpenAI chat-completions shape. Routes to a worker by model. Runs a real container.',
    overview:
      'The OpenAI chat-completions endpoint. Set `model` to the target agent’s peer id (from /v1/models), that is how you address a specific agent. The Boss dispatches the conversation to that worker and returns an OpenAI-shaped completion; with stream:true you get token deltas as Server-Sent `data:` chunks ending in [DONE]. (An agent display name is also accepted as a routing shortcut, but peer ids are the unique, stable identifier and what you should use.)',
    whenToUse:
      'Drop AgentFM into any tool that already speaks OpenAI chat, point the SDK’s base_url at the Boss and pass an agent peer id as the model.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'body', loc: 'body', required: true, example: CHAT_BODY, description: 'model = the agent peer id (from /v1/models); messages as usual. Set stream:true for token streaming.' },
    ],
    exampleResponse: { id: 'chatcmpl-1', object: 'chat.completion', choices: [{ index: 0, message: { role: 'assistant', content: '…' }, finish_reason: 'stop' }] },
    responseFields: [
      { name: 'id', type: 'string', description: 'Completion id.' },
      { name: 'choices[].message.content', type: 'string', description: 'The assistant reply (non-streaming).' },
      { name: 'choices[].delta.content', type: 'string', description: 'Incremental token text when stream:true.' },
      { name: 'finish_reason', type: 'string', description: '"stop" on normal completion.' },
    ],
    errors: [
      { status: '400', when: 'Body is missing model or messages.' },
      { status: '404', when: 'No worker advertises the requested model.' },
    ],
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
    overview:
      'The legacy OpenAI text-completions endpoint: a single `prompt` instead of `messages`. Routes to a worker by `model` and supports streaming. Prefer /v1/chat/completions for new code.',
    whenToUse:
      'Compatibility with older OpenAI clients or simple single-prompt completion calls.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'body', loc: 'body', required: true, example: { model: EXAMPLE_PEER, prompt: 'Hello', stream: false }, description: 'model = the agent peer id (from /v1/models); prompt is the text. Set stream:true for token streaming.' },
    ],
    exampleResponse: { id: 'cmpl-1', object: 'text_completion', choices: [{ index: 0, text: '…', finish_reason: 'stop' }] },
    responseFields: [
      { name: 'choices[].text', type: 'string', description: 'The generated completion text.' },
      { name: 'finish_reason', type: 'string', description: '"stop" on normal completion.' },
    ],
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
    overview:
      'A compact reputation card for one peer: its current trust score, how many ratings and comments exist about it, and cached identity metadata (agent name, author). This is the headline view; /reputation and /log drill deeper.',
    whenToUse:
      'Show a peer’s standing before dispatching to it, or link from a worker card to its profile.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Target peer id.' },
    ],
    exampleResponse: { peer_id: '12D3KooW…', score: 0.42, ratings: 12, comments: 3, agent: 'HR Agent' },
    responseFields: [
      { name: 'peer_id', type: 'string', description: 'The peer this summary is about.' },
      { name: 'score', type: 'number', description: 'Aggregate trust score in [-1, 1].' },
      { name: 'ratings / comments', type: 'number', description: 'How many signed ratings / comments exist about this peer.' },
    ],
    errors: [{ status: '400', when: 'peerId is not a valid libp2p peer id.' }],
    sideEffect: 'none',
  },
  {
    id: 'peer-reputation',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId/reputation',
    summary: 'Peer honesty scores',
    description: 'Honesty/EigenTrust scores, equivocator flag and rating count for one peer.',
    overview:
      'The detailed reputation breakdown for a peer: its EigenTrust-derived honesty score, whether it has been flagged as an equivocator (caught double-signing), and the number of ratings feeding the score. This is the data the dispatch reputation-floor check uses.',
    whenToUse:
      'Decide whether a peer clears your trust threshold, or surface an equivocation warning.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Target peer id.' },
    ],
    exampleResponse: { peer_id: '12D3KooW…', honesty_score: 0.42, equivocator: false, ratings: 12 },
    responseFields: [
      { name: 'honesty_score', type: 'number', description: 'EigenTrust-style score in [-1, 1].' },
      { name: 'equivocator', type: 'boolean', description: 'True if the peer was caught signing conflicting log heads.' },
      { name: 'ratings', type: 'number', description: 'Number of ratings contributing to the score.' },
    ],
    sideEffect: 'none',
  },
  {
    id: 'peer-log',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId/log',
    summary: 'Paginated ledger entries',
    description: 'Signed ledger entries (ratings + comments) recorded about a peer. Page with limit and offset.',
    overview:
      'The append-only ledger of signed entries about a peer, every rating and comment, newest first, with the rater’s id and trust status. Each entry is individually verifiable (see /proof). Page through with limit and offset.',
    whenToUse:
      'Render a peer’s activity/history feed, or audit who has rated a worker and how.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Target peer id.' },
      { name: 'limit', loc: 'query', required: false, example: '50', description: 'Max entries to return (default server-side cap applies).' },
      { name: 'offset', loc: 'query', required: false, example: '0', description: 'Entries to skip, for pagination.' },
    ],
    exampleResponse: {
      subject: '12D3KooW…',
      limit: 50,
      offset: 0,
      returned: 2,
      entries: [
        { received_at: '2026-06-20T15:50:00Z', kind: 'Rating', entry_hash: '9f2c4e…(64 hex)', rater_peer_id: '12D3KooW…', rater_status: 'verified', rater_honesty_score: 0.42, score: 0.3 },
        { received_at: '2026-06-20T15:49:00Z', kind: 'Comment', entry_hash: '7a1b8d…(64 hex)', rater_peer_id: '12D3KooW…', rater_status: 'verified', rater_honesty_score: 0.42, language: 'en', text_cid: 'a1b2c3…' },
      ],
    },
    responseFields: [
      { name: 'subject / limit / offset / returned', type: 'mixed', description: 'Echoed query plus how many entries this page returned.' },
      { name: 'entries[].kind', type: 'string', description: '"Rating" or "Comment".' },
      { name: 'entries[].entry_hash', type: 'string (hex)', description: 'Hex leaf hash of the entry, pass directly to GET /v1/peers/:peerId/proof?entry=<entry_hash> to fetch its Merkle inclusion proof.' },
      { name: 'entries[].rater_peer_id', type: 'string', description: 'Who signed the entry.' },
      { name: 'entries[].rater_status / rater_honesty_score', type: 'string / number', description: 'Whether the rater is verified, and their trust score.' },
      { name: 'entries[].score', type: 'number', description: 'Numeric rating (Rating entries; omitted for comment-only).' },
      { name: 'entries[].text_cid', type: 'string (hex)', description: 'Comment entries only, the cid of the comment body. Pass it to GET /comments/:cid to fetch the text.' },
    ],
    notes: 'This is the only HTTP endpoint that surfaces a comment’s text_cid. Each entry also carries its entry_hash (the Merkle leaf hash), pass it to GET /proof?entry= to obtain that entry’s inclusion proof.',
    sideEffect: 'none',
  },
  {
    id: 'peer-proof',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId/proof',
    summary: 'Merkle audit proof',
    description: 'Merkle inclusion proof for a single ledger entry, so a client can verify it against the signed log head.',
    overview:
      'Returns a Merkle inclusion proof for one ledger entry: its position, the audit path (sibling hashes up to the root), and the current signed log head (root hash, tree size, witness count). A client can verify an entry is really in the log without trusting the server, by folding the audit path up to the root and checking it matches a witness-co-signed head.',
    whenToUse:
      'Independently verify a rating/comment, or build a trust-minimised auditor.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Target peer id.' },
      { name: 'entry', loc: 'query', required: true, example: '9f2c4e…(64 hex)', description: 'A 64-character hex leaf hash of the entry to prove. Get it from GET /v1/peers/:peerId/log, each entry now carries an entry_hash field; pass that value here.' },
    ],
    exampleResponse: {
      entry_hash: '9f2c4e…',
      position: 7,
      audit_path: ['ab12…', 'cd34…'],
      head: { tree_size: 42, root_hash: 'ff00…', witness_count: 1, signed_at: '2026-06-20T15:50:00Z' },
    },
    responseFields: [
      { name: 'entry_hash', type: 'string (hex)', description: 'Echo of the proven leaf hash.' },
      { name: 'position', type: 'number', description: 'Zero-based leaf index of the entry in the log.' },
      { name: 'audit_path[]', type: 'string[] (hex)', description: 'Ordered sibling hashes from the entry up to the root.' },
      { name: 'head.root_hash', type: 'string (hex)', description: 'Merkle root the proof verifies against.' },
      { name: 'head.tree_size', type: 'number', description: 'Number of leaves in the log at proof time.' },
      { name: 'head.witness_count', type: 'number', description: 'How many witnesses co-signed this head (prefer > 0).' },
    ],
    errors: [
      { status: '400', when: 'Missing entry, or entry is not a 64-char hex string.' },
      { status: '404', when: 'No entry with that hash exists in the peer’s log (entry_not_found).' },
      { status: '503', when: 'Ledger not wired on this boss (ledger_unavailable).' },
    ],
    notes: 'Discover an entry’s leaf hash from GET /v1/peers/:peerId/log, each entry carries an entry_hash field you can pass straight to ?entry=. Computing SHA-256 over the entry’s canonical signed bytes, or fetching over the P2P ledger protocol, remain alternatives.',
    sideEffect: 'none',
  },
  {
    id: 'peer-self-comment',
    group: 'AgentFM-native',
    method: 'POST',
    path: '/v1/peers/:peerId/comments/self',
    summary: 'Submit self-signed feedback',
    description: 'Leave a comment that the Boss signs with its OWN libp2p identity. No caller signature needed, the rater is implicitly this node.',
    overview:
      'The easy way to leave feedback: send just the text (and optionally a rating), and the Boss signs the entry with its own libp2p identity (b.node.Host.ID()) before appending it to the ledger and gossiping it to witnesses. The entry is attributed to this node, there is no rater_peer_id or signature field, so you can’t spoof the author. This is what the desktop "Leave feedback" flow uses.',
    whenToUse:
      'Record your own rating/comment about a worker after a task, no key handling or client-side signing needed.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Subject, the peer the feedback is about.' },
      {
        name: 'body',
        loc: 'body',
        required: true,
        example: { text: 'Reliable worker.', language: 'en', rating: 0.3 },
        description:
          'text (required) = the comment. language optional (e.g. "en"). rating optional, a finite number in [-1.0, +1.0], when present, the Boss ALSO appends a paired Rating entry (dimension "honesty"). Optional attached_rating_hash (hex) links an existing rating instead.',
      },
    ],
    exampleResponse: { cid: 'a1b2c3…', ledger_hash: '9f2c4e…' },
    responseFields: [
      { name: 'cid', type: 'string (hex)', description: 'Content id of the stored comment body (use with GET /comments/:cid).' },
      { name: 'ledger_hash', type: 'string (hex)', description: 'Hash of the appended ledger entry.' },
    ],
    errors: [
      { status: '400', when: 'Missing text, body too large, or rating not in [-1, 1] (bad_rating).' },
      { status: '503', when: 'Ledger / comments store / libp2p host not wired on this boss.' },
    ],
    notes: 'The rater is implicit, this node’s identity (see /v1/about → boss_peer_id). The Boss signs with its own key, so the attribution is cryptographic, not a self-declared field.',
    sideEffect: 'signed',
  },
  {
    id: 'peer-comment-body',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId/comments/:cid',
    summary: 'Fetch comment body (text)',
    description: 'Returns the plain-text body of a content-addressed comment by its CID.',
    overview:
      'Ledger entries store only a content id (CID) for a comment’s text; this endpoint resolves that CID to the actual body as plain text. The CID comes from a /log entry’s text_cid field.',
    whenToUse:
      'Lazily load a comment’s text when expanding a ledger entry.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Peer the comment is about.' },
      { name: 'cid', loc: 'path', required: true, example: 'bafy…', description: 'Content id of the comment. Get it from GET /v1/peers/:peerId/log, each Comment entry carries a text_cid field; pass that value here.' },
    ],
    exampleResponse: 'Reliable worker, fast turnaround.',
    responseFields: [{ name: '(body)', type: 'text/plain', description: 'The raw comment text.' }],
    errors: [{ status: '404', when: 'No comment body stored for that CID.' }],
    notes: 'Returns the raw body as text/plain; charset=utf-8. For a structured envelope, use the .json variant.',
    sideEffect: 'none',
  },
  {
    id: 'peer-comment-body-json',
    group: 'AgentFM-native',
    method: 'GET',
    path: '/v1/peers/:peerId/comments/:cid.json',
    summary: 'Fetch comment body (JSON)',
    description: 'Same comment body as the text endpoint, wrapped in a JSON envelope.',
    overview:
      'Identical content to the plain-text comment endpoint, but returned as a JSON object so clients that prefer structured responses can avoid sniffing content types.',
    whenToUse:
      'When your client wants JSON everywhere instead of a bare text/plain body.',
    auth: AUTH_LOOPBACK,
    params: [
      { name: 'peerId', loc: 'path', required: true, example: '12D3KooW…', description: 'Peer the comment is about.' },
      { name: 'cid', loc: 'path', required: true, example: 'bafy…', description: 'Content id of the comment. Get it from GET /v1/peers/:peerId/log, each Comment entry carries a text_cid field; pass that value here.' },
    ],
    exampleResponse: { cid: 'bafy…', body: 'Reliable worker, fast turnaround.', language: 'en' },
    responseFields: [
      { name: 'cid', type: 'string', description: 'The requested content id.' },
      { name: 'body', type: 'string', description: 'The comment text.' },
      { name: 'language', type: 'string', description: 'BCP-47-ish language tag for the body.' },
    ],
    errors: [{ status: '404', when: 'No comment body stored for that CID.' }],
    notes: 'Content-Type is application/json, unlike the plain /comments/:cid variant which returns text/plain.',
    sideEffect: 'none',
  },
  {
    id: 'metrics',
    group: 'System',
    method: 'GET',
    path: '/metrics',
    summary: 'Prometheus metrics',
    description: 'Prometheus text exposition of gateway + mesh counters (tasks, errors, auth, artifact bytes). Unauthenticated.',
    overview:
      'The Prometheus scrape endpoint, returning the standard text exposition format. Exposes gateway and mesh counters, tasks by status, stream errors by protocol, auth attempts, artifact bytes, plus the Go process metrics, so you can wire AgentFM into Prometheus/Grafana.',
    whenToUse:
      'Scrape with Prometheus, or spot-check counters during debugging.',
    auth: AUTH_PUBLIC,
    params: [],
    exampleResponse: '# HELP agentfm_tasks_total Total tasks dispatched\n# TYPE agentfm_tasks_total counter\nagentfm_tasks_total{status="ok"} 4\nagentfm_tasks_total{status="error"} 0',
    responseFields: [
      { name: 'agentfm_tasks_total', type: 'counter', description: 'Tasks dispatched, labelled by status.' },
      { name: 'agentfm_stream_errors_total', type: 'counter', description: 'Stream errors, labelled by protocol.' },
      { name: 'agentfm_artifacts_built_total', type: 'counter', description: 'Artifact zips harvested from workers.' },
    ],
    notes: 'Content-Type is text/plain in Prometheus exposition format, not JSON.',
    sideEffect: 'none',
  },
]

export type FormValues = Record<string, string>

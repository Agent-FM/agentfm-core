import type { AboutResponse, WorkerListResponse, PeerSummary, PeerLogResponse } from '../types/api'

const DEFAULT_PORT = 8080
let port = DEFAULT_PORT
const baseURL = () => `http://127.0.0.1:${port}`

export function setApiPort(p: number) {
  port = p
}

export async function loadApiPortFromSettings() {
  try {
    const stored = await window.api?.settings.get<number>('apiPort')
    if (typeof stored === 'number') port = stored
  } catch {}
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${baseURL()}${path}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new ApiError(res.status, text || res.statusText)
  }
  return res.json()
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${baseURL()}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new ApiError(res.status, text || res.statusText)
  }
  return res.json()
}

export interface SelfCommentResponse {
  cid: string
  ledger_hash: string
}

export const api = {
  about: () => getJSON<AboutResponse>('/v1/about'),
  health: () => getJSON<{ status: string; online_workers: number }>('/health'),
  workers: (includeOffline = false) =>
    getJSON<WorkerListResponse>(`/api/workers${includeOffline ? '?include_offline=true' : ''}`),
  peer: (peerId: string) => getJSON<PeerSummary>(`/v1/peers/${peerId}`),
  peerLog: (
    peerId: string,
    { limit = 50, offset = 0 }: { limit?: number; offset?: number } = {},
  ) => getJSON<PeerLogResponse>(`/v1/peers/${peerId}/log?limit=${limit}&offset=${offset}`),
  commentBody: (peerId: string, cid: string) =>
    getJSON<{ cid: string; body: string; language: string }>(
      `/v1/peers/${peerId}/comments/${cid}.json`,
    ),
  submitSelfComment: (
    peerId: string,
    body: { text: string; language?: string; attached_rating_hash?: string },
  ) =>
    postJSON<SelfCommentResponse>(
      `/v1/peers/${peerId}/comments/self`,
      body,
    ),

  // Streaming endpoints — return a Response guaranteed to be ok with a body.
  // On non-2xx, ApiError is thrown with the error body text.
  execute: async (
    body: {
      worker_id: string
      prompt: string
      task_id?: string
      feedback?: string
      feedback_rating?: number
    },
    signal?: AbortSignal,
  ) => {
    const res = await fetch(`${baseURL()}/api/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new ApiError(res.status, text || res.statusText)
    }
    return res
  },

  chatCompletion: async (body: unknown, signal?: AbortSignal) => {
    const res = await fetch(`${baseURL()}/v1/chat/completions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new ApiError(res.status, text || res.statusText)
    }
    return res
  },
}

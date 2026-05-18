export interface AboutResponse {
  boss_peer_id: string
  relay_peer_id: string
  relay_multiaddr: string
  reputation_floor: number
  ledger_tree_size: number
  version: string
  uptime_seconds: number
}

export interface WorkerProfile {
  peer_id: string
  name: string
  status: string
  online: boolean
  last_seen: string | null
  honesty_score: number
  is_equivocator: boolean
  dispatch_allowed: boolean
  dispatch_refuse_reason?: string
  agent_image_ref?: string
  agent_image_digest?: string
  agent_capability?: string
  current_tasks: number
  max_tasks: number
  cpu_usage_pct: number
  ram_free_gb: number
  has_gpu: boolean
  model?: string
}

export interface WorkerListResponse {
  success: boolean
  online_count: number
  offline_count: number
  agents: WorkerProfile[]
}

export interface PeerEntry {
  received_at: string
  kind: 'Rating' | 'Comment'
  rater_peer_id: string
  rater_status: 'verified' | 'unverified'
  rater_honesty_score: number
  dimension?: string
  score?: number
  context?: string
  language?: string
  text_cid?: string
}

export interface PeerLogResponse {
  subject: string
  limit: number
  offset: number
  returned: number
  entries: PeerEntry[]
}

export interface PeerSummary {
  peer_id: string
  agent_name: string
  online: boolean
  last_seen: string | null
  honesty_score: number
  is_equivocator: boolean
  dispatch_allowed: boolean
  dispatch_refuse_reason?: string
  entries_count: number
  last_entry_at: string | null
  advertised_image_ref?: string
  advertised_image_digest?: string
  advertised_capability?: string
  rater_summary: {
    verified_raters_count: number
    unverified_raters_count: number
  }
}

export interface SseEvent {
  type: 'worker_online' | 'worker_offline' | 'entry_appended' | 'equivocator_marked'
  data: unknown
}

import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { WorkerProfile } from '../types/api'

export interface PeerIdentity {
  name?: string
  author?: string
  description?: string
  agent_capability?: string
  agent_image_ref?: string
}

interface PeerIdentityStore {
  byPeerId: Record<string, PeerIdentity>
  remember: (workers: WorkerProfile[]) => void
}

// Bound the persisted cache so it doesn't grow forever across restarts on a
// churning mesh. Keeps the most-recently-inserted MAX_PEERS entries.
const MAX_PEERS = 500

function hasContent(w: WorkerProfile): boolean {
  return (
    !!(w.name && w.name.trim()) ||
    !!(w.author && w.author.trim()) ||
    !!(w.description && w.description.trim()) ||
    !!(w.agent_capability && w.agent_capability.trim()) ||
    !!(w.agent_image_ref && w.agent_image_ref.trim())
  )
}

export const usePeerIdentityCache = create<PeerIdentityStore>()(
  persist(
    (set) => ({
      byPeerId: {},
      remember: (workers) =>
        set((state) => {
          let changed = false
          const next = { ...state.byPeerId }
          for (const w of workers) {
            if (!hasContent(w)) continue
            const prev = next[w.peer_id]
            const merged: PeerIdentity = {
              name: w.name?.trim() || prev?.name,
              author: w.author?.trim() || prev?.author,
              description: w.description?.trim() || prev?.description,
              agent_capability: w.agent_capability?.trim() || prev?.agent_capability,
              agent_image_ref: w.agent_image_ref?.trim() || prev?.agent_image_ref,
            }
            if (
              merged.name !== prev?.name ||
              merged.author !== prev?.author ||
              merged.description !== prev?.description ||
              merged.agent_capability !== prev?.agent_capability ||
              merged.agent_image_ref !== prev?.agent_image_ref
            ) {
              next[w.peer_id] = merged
              changed = true
            }
          }
          if (!changed) return state
          const keys = Object.keys(next)
          if (keys.length > MAX_PEERS) {
            const trimmed: Record<string, PeerIdentity> = {}
            for (const k of keys.slice(keys.length - MAX_PEERS)) trimmed[k] = next[k]
            return { byPeerId: trimmed }
          }
          return { byPeerId: next }
        }),
    }),
    { name: 'agentfm-peer-identity' },
  ),
)

export function mergeWithCache(
  w: WorkerProfile,
  cache: Record<string, PeerIdentity>,
): WorkerProfile {
  const cached = cache[w.peer_id]
  if (!cached) return w
  return {
    ...w,
    name: w.name?.trim() ? w.name : cached.name ?? w.name,
    author: w.author?.trim() ? w.author : cached.author ?? w.author,
    description: w.description?.trim() ? w.description : cached.description ?? w.description,
    agent_capability:
      w.agent_capability?.trim() ? w.agent_capability : cached.agent_capability ?? w.agent_capability,
    agent_image_ref:
      w.agent_image_ref?.trim() ? w.agent_image_ref : cached.agent_image_ref ?? w.agent_image_ref,
  }
}

import { shortenPeerID } from './peer'
import type { PeerIdentity } from './peerIdentityCache'

interface NameSource {
  name?: string
  agent_capability?: string
  agent_image_ref?: string
  peer_id: string
}

export function displayName(w: NameSource, cached?: PeerIdentity): string {
  if (w.name && w.name.trim()) return w.name.trim()
  if (cached?.name && cached.name.trim()) return cached.name.trim()
  if (w.agent_capability && w.agent_capability.trim()) return w.agent_capability.trim()
  if (cached?.agent_capability && cached.agent_capability.trim()) return cached.agent_capability.trim()
  const ref = w.agent_image_ref?.trim() || cached?.agent_image_ref?.trim()
  if (ref) {
    const tail = ref.split('/').pop() ?? ref
    const noTag = tail.split(':')[0]
    if (noTag) return noTag
  }
  if (w.peer_id) return shortenPeerID(w.peer_id, 6, 5)
  return '(unknown agent)'
}

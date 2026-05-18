import { shortenPeerID } from './peer'

interface NameSource {
  name?: string
  agent_capability?: string
  agent_image_ref?: string
  peer_id: string
}

export function displayName(w: NameSource): string {
  if (w.name && w.name.trim()) return w.name.trim()
  if (w.agent_capability && w.agent_capability.trim()) return w.agent_capability.trim()
  if (w.agent_image_ref && w.agent_image_ref.trim()) {
    const tail = w.agent_image_ref.split('/').pop() ?? w.agent_image_ref
    const noTag = tail.split(':')[0]
    if (noTag) return noTag
  }
  if (w.peer_id) return shortenPeerID(w.peer_id, 6, 5)
  return '(unknown agent)'
}

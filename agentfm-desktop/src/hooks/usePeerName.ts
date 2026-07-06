import { useWorkers } from '../lib/query'
import { displayName } from '../lib/displayName'
import { shortenPeerID } from '../lib/peer'

export function usePeerName(peerId: string): string {
  const { data } = useWorkers(true)
  const w = data?.agents.find((a) => a.peer_id === peerId)
  if (w) return displayName(w)
  return shortenPeerID(peerId, 8, 6)
}

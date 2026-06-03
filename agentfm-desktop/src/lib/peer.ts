export function shortenPeerID(peerId: string, head = 6, tail = 5): string {
  if (peerId.length <= head + tail + 3) return peerId
  return `${peerId.slice(0, head)}…${peerId.slice(-tail)}`
}

export function shortenDigest(digest: string, n = 12): string {
  if (!digest) return ''
  const stripped = digest.startsWith('sha256:') ? digest.slice(7) : digest
  return `sha256:${stripped.slice(0, n)}…`
}

export function compactAge(date: string | number | Date): string {
  const d = new Date(date)
  const seconds = Math.floor((Date.now() - d.getTime()) / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

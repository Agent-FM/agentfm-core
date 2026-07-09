import { useState } from 'react'
import { Copy, Check, Lock } from 'lucide-react'
import { toast } from 'sonner'
import { shortenPeerID } from '../../lib/peer'

interface Props {
  peerId: string
  mode: 'public' | 'private'
}

export function RelayPill({ peerId, mode }: Props) {
  const isPrivate = mode === 'private'
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(peerId)
      setCopied(true)
      setTimeout(() => setCopied(false), 800)
      toast.success('Peer ID copied')
    } catch {
      toast.error('Could not copy')
    }
  }

  return (
    <button
      onClick={copy}
      title={`${peerId}, click to copy`}
      className="group inline-flex items-center gap-1.5 h-[20px] font-mono text-2xs tabular-nums text-text-1 bg-raised border border-border-1 rounded-ctl px-2 transition-colors hover:bg-control"
    >
      <span className="w-[5px] h-[5px] rounded-full bg-ok" />
      {isPrivate && <Lock size={10} strokeWidth={1.5} />}
      {shortenPeerID(peerId, 6, 5)}
      {copied ? (
        <Check size={12} strokeWidth={1.5} className="text-ok" />
      ) : (
        <Copy size={12} className="opacity-0 group-hover:opacity-70 transition-opacity" />
      )}
    </button>
  )
}

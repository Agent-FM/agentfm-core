import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { toast } from 'sonner'
import { shortenPeerID } from '../../lib/peer'

interface Props {
  peerId: string
  mode: 'public' | 'private'
}

export function RelayPill({ peerId, mode }: Props) {
  const isPrivate = mode === 'private'
  const dotColor = '#F7931E'
  const textColor = '#FCD9A8'
  const bg = 'rgba(247,147,30,.06)'
  const border = 'rgba(247,147,30,.25)'
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
      title={`${peerId} — click to copy`}
      className="group inline-flex items-center gap-1.5 font-mono transition-colors hover:brightness-125"
      style={{
        fontSize: 11,
        color: textColor,
        background: bg,
        border: `1px solid ${border}`,
        padding: '5px 10px',
        borderRadius: 999,
      }}
    >
      <span
        className="w-[5px] h-[5px] rounded-full animate-pulse-cyan"
        style={{ background: dotColor, boxShadow: `0 0 6px ${dotColor}` }}
      />
      {isPrivate ? '🔒 ' : ''}{shortenPeerID(peerId, 6, 5)}
      {copied ? (
        <Check size={12} className="text-accent" />
      ) : (
        <Copy size={12} className="opacity-0 group-hover:opacity-70 transition-opacity" />
      )}
    </button>
  )
}

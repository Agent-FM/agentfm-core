import { shortenPeerID } from '../../lib/peer'

interface Props {
  peerId: string
  mode: 'public' | 'private'
}

export function RelayPill({ peerId, mode }: Props) {
  const isPrivate = mode === 'private'
  const dotColor = isPrivate ? '#a855f7' : '#22d3ee'
  const textColor = isPrivate ? '#d8b4fe' : '#a5f3fc'
  const bg = isPrivate ? 'rgba(168,85,247,.06)' : 'rgba(34,211,238,.06)'
  const border = isPrivate ? 'rgba(168,85,247,.25)' : 'rgba(34,211,238,.25)'
  return (
    <div
      className="inline-flex items-center gap-1.5 font-mono"
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
    </div>
  )
}

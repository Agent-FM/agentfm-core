import { motion } from 'framer-motion'
import type { ChatMessage } from '../../types/chat'
import { shortenPeerID, compactAge } from '../../lib/peer'

export function MessageBubble({ msg, streaming }: { msg: ChatMessage; streaming?: boolean }) {
  const isUser = msg.role === 'user'
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className={`max-w-[80%] min-w-0 overflow-hidden px-4 py-3 rounded-2xl text-sm leading-relaxed ${
        isUser
          ? 'self-end bg-accent-bg border border-accent/35 text-text-0'
          : 'self-start bg-bg-1 border border-accent2/20 text-text-0'
      }`}
    >
      <div className="text-2xs text-text-2 mb-1.5 flex items-center gap-2">
        {isUser
          ? <span>You</span>
          : <span className="font-mono">{msg.rater_peer_id ? shortenPeerID(msg.rater_peer_id, 6, 5) : 'agent'}</span>}
        <span className="ml-auto">{streaming && !isUser ? 'streaming…' : compactAge(msg.timestamp) + ' ago'}</span>
      </div>
      <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
        {msg.content}
        {streaming && !isUser && (
          <span className="inline-block w-[3px] h-4 bg-accent ml-0.5 align-middle animate-pulse-cyan shadow-[0_0_8px_#22d3ee]" />
        )}
      </div>
    </motion.div>
  )
}

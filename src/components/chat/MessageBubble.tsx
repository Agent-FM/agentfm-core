import { motion } from 'framer-motion';
import type { ChatMessage } from '../../types/chat';
import { shortenPeerID, compactAge } from '../../lib/peer';

export function MessageBubble({
  msg,
  streaming,
}: {
  msg: ChatMessage;
  streaming?: boolean;
}) {
  const isUser = msg.role === 'user';
  // `layout` is intentionally only applied on entry. Re-running layout
  // measurements on every streamed token causes jank with long responses.
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className={`max-w-[75%] min-w-0 overflow-hidden px-3.5 py-3 rounded-xl text-sm leading-relaxed ${
        isUser
          ? 'self-end bg-accent-bg border border-accent/30 text-text-0'
          : 'self-start bg-bg-1 border border-border-0 text-text-0'
      }`}
    >
      <div className="text-[11px] text-text-2 mb-1 flex items-center gap-2">
        {isUser ? (
          <span>You</span>
        ) : (
          <span className="font-mono">
            {msg.rater_peer_id
              ? shortenPeerID(msg.rater_peer_id, 6, 5)
              : 'agent'}
          </span>
        )}
        <span className="ml-auto">
          {streaming && !isUser
            ? 'streaming…'
            : compactAge(msg.timestamp) + ' ago'}
        </span>
      </div>
      <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
        {msg.content}
        {streaming && !isUser && (
          <span className="inline-block w-2 h-3.5 bg-accent ml-0.5 animate-blink align-middle" />
        )}
      </div>
    </motion.div>
  );
}

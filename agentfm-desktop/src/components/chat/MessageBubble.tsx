import { ExternalLink, MessageSquare } from 'lucide-react'
import type { ChatMessage } from '../../types/chat'
import { shortenPeerID, compactAge } from '../../lib/peer'
import { Card } from '../primitives/Card'
import { Avatar } from '../primitives/Avatar'
import { useUIStore } from '../../lib/store'

export function MessageBubble({ msg, streaming }: { msg: ChatMessage; streaming?: boolean }) {
  const isUser = msg.role === 'user'
  const showArtifact = !isUser && msg.has_artifact && msg.task_id
  const showFeedback = !isUser && !streaming && msg.rater_peer_id && msg.task_id && msg.content.trim().length > 0
  const openFeedback = useUIStore((s) => s.openFeedback)

  return (
    <div className={`flex flex-col gap-2 max-w-[78%] min-w-0 ${isUser ? 'self-end items-end' : 'self-start items-start'}`}>
      {isUser ? (
        <div
          className="px-4 py-3 rounded-[18px_18px_4px_18px] text-[15px] leading-[1.55] w-full
            bg-accent-soft border border-accent/20 text-text-0"
        >
          <div className="text-[10px] uppercase tracking-[0.14em] text-text-2 font-mono font-bold mb-1">
            YOU
          </div>
          <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
            {msg.content}
          </div>
        </div>
      ) : (
        <Card className="rounded-[18px_18px_18px_4px] px-[18px] py-4 w-full">
          <div className="flex items-center gap-2 text-[10px] uppercase tracking-[0.14em] text-text-2 font-mono font-bold mb-2">
            <span>AGENT</span>
            {msg.rater_peer_id && (
              <span className="font-mono text-accent tracking-normal normal-case font-semibold tabular-nums">
                {shortenPeerID(msg.rater_peer_id, 6, 5)}
              </span>
            )}
            <span className="ml-auto normal-case font-normal tracking-normal text-text-3 font-mono tabular-nums">
              {streaming ? (
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-accent animate-pulse" />
                  <span>streaming</span>
                </span>
              ) : (
                <span>{compactAge(msg.timestamp)} ago</span>
              )}
            </span>
          </div>
          <div
            className="whitespace-pre-wrap text-[15px] leading-[1.6] text-text-0"
            style={{ overflowWrap: 'anywhere' }}
          >
            {msg.content}
            {streaming && (
              <span className="inline-block w-[3px] h-4 ml-0.5 align-middle bg-accent animate-pulse" />
            )}
          </div>
        </Card>
      )}

      {showArtifact && (
        <div
          className="flex items-center gap-3.5 w-full rounded-2xl px-3.5 py-3
            bg-accent-soft border border-accent/20"
        >
          <Avatar size="sm" emoji="📦" />
          <div className="flex-1 min-w-0">
            <div className="text-[10px] uppercase tracking-[0.14em] text-text-2 font-mono font-bold">
              Artifact ready
            </div>
            <div className="text-[14px] font-mono font-semibold text-text-0 truncate tabular-nums" title={msg.task_id}>
              {msg.task_id}.zip
            </div>
          </div>
          <button
            onClick={() => { void window.api.app.openArtifact(msg.task_id!).catch(() => {}) }}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-semibold
              text-accent bg-accent-soft border border-accent/20 hover:bg-bg-1 transition-colors"
          >
            <ExternalLink size={11} />
            <span>Show in Finder</span>
          </button>
        </div>
      )}

      {showFeedback && (
        <button
          onClick={() => openFeedback(msg.rater_peer_id!, msg.task_id!)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px]
            text-text-1 hover:text-text-0 border border-border-0
            bg-bg-1 hover:bg-bg-2 transition-colors"
        >
          <MessageSquare size={11} />
          <span>Rate this response</span>
        </button>
      )}
    </div>
  )
}

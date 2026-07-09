import { ExternalLink, MessageSquare, Package } from 'lucide-react'
import type { ChatMessage } from '../../types/chat'
import { shortenPeerID, compactAge } from '../../lib/peer'
import { Avatar } from '../primitives/Avatar'
import { StatusDot } from '../primitives/StatusDot'
import { useUIStore } from '../../lib/store'

export function MessageBubble({ msg, streaming }: { msg: ChatMessage; streaming?: boolean }) {
  const isUser = msg.role === 'user'
  const showArtifact = !isUser && msg.has_artifact && msg.task_id
  const showFeedback = !isUser && !streaming && msg.rater_peer_id && msg.task_id && msg.content.trim().length > 0
  const openFeedback = useUIStore((s) => s.openFeedback)

  return (
    <div className={`flex flex-col gap-1.5 max-w-[78%] min-w-0 ${isUser ? 'self-end items-end' : 'self-start items-start'}`}>
      {isUser ? (
        <div className="px-2.5 py-1.5 rounded-ctl text-sm leading-relaxed w-full bg-raised border border-border-0 text-text-0">
          <div className="text-2xs uppercase tracking-[0.06em] text-text-1 font-medium mb-0.5">
            YOU
          </div>
          <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
            {msg.content}
          </div>
        </div>
      ) : (
        <div className="w-full min-w-0">
          <div className="flex items-center gap-2 text-2xs uppercase tracking-[0.06em] text-text-1 font-medium mb-1">
            <span>AGENT</span>
            {msg.rater_peer_id && (
              <span className="font-mono text-accent tracking-normal normal-case tabular-nums">
                {shortenPeerID(msg.rater_peer_id, 6, 5)}
              </span>
            )}
            <span className="ml-auto normal-case tracking-normal text-text-2 font-mono tabular-nums">
              {streaming ? (
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot tone="accent" size="sm" pulse />
                  <span>streaming</span>
                </span>
              ) : (
                <span>{compactAge(msg.timestamp)} ago</span>
              )}
            </span>
          </div>
          <div
            className="console-well mono-console whitespace-pre-wrap p-2.5"
            style={{ overflowWrap: 'anywhere' }}
          >
            {msg.content}
            {streaming && (
              <span className="inline-block w-[3px] h-3 ml-0.5 align-middle bg-accent" />
            )}
          </div>
        </div>
      )}

      {showArtifact && (
        <div className="flex items-center gap-2.5 w-full rounded-ctl px-2.5 py-2 bg-raised border border-border-0">
          <Avatar size="sm">
            <Package size={14} strokeWidth={1.5} />
          </Avatar>
          <div className="flex-1 min-w-0">
            <div className="text-2xs uppercase tracking-[0.06em] text-text-1 font-medium">
              Artifact ready
            </div>
            <div className="font-mono text-xs text-text-0 truncate tabular-nums" title={msg.task_id}>
              {msg.task_id}.zip
            </div>
          </div>
          <button
            onClick={() => { void window.api.app.openArtifact(msg.task_id!).catch(() => {}) }}
            className="inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-ctl text-sm
              text-text-0 bg-[#3A3A3E] hover:bg-[#454549] active:bg-[#333337] transition-colors duration-150"
          >
            <ExternalLink size={11} strokeWidth={1.5} />
            <span>Show in Finder</span>
          </button>
        </div>
      )}

      {showFeedback && (
        <button
          onClick={() => openFeedback(msg.rater_peer_id!, msg.task_id!)}
          className="inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-ctl text-xs
            text-text-1 hover:text-text-0 bg-transparent border border-border-1 hover:bg-white/[0.05]
            transition-colors duration-150"
        >
          <MessageSquare size={11} strokeWidth={1.5} />
          <span>Rate this response</span>
        </button>
      )}
    </div>
  )
}

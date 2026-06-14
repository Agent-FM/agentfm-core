import { Zap, MessageSquare, ScrollText } from 'lucide-react'
import type { WorkerProfile } from '../types/api'
import { NeonCard } from './primitives/NeonCard'
import { Avatar } from './primitives/Avatar'
import { Meter } from './primitives/Meter'
import { Button } from './primitives/Button'
import { Badge } from './primitives/Badge'
import { shortenPeerID } from '../lib/peer'
import { displayName } from '../lib/displayName'
import { useUIStore } from '../lib/store'
import { usePeerIdentityCache } from '../lib/peerIdentityCache'
import { useNavigate } from 'react-router-dom'

interface Props {
  worker: WorkerProfile
}

function honestyTone(score: number): 'lime' | 'rose' | 'cyan' {
  if (score >= 0.3) return 'lime'
  if (score <= -0.5) return 'rose'
  return 'cyan'
}

function emojiFor(name: string): string {
  const first = (name || '').trim().toLowerCase()
  if (first.includes('hr')) return '🤖'
  if (first.includes('write')) return '📝'
  if (first.includes('data')) return '📊'
  if (first.includes('code')) return '💻'
  return '🤖'
}

function formatLastSeen(iso: string | null): string {
  if (!iso) return 'never seen'
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return 'never seen'
  const diff = Date.now() - t
  if (diff < 60_000) return 'last seen just now'
  if (diff < 3600_000) return `last seen ${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `last seen ${Math.floor(diff / 3_600_000)}h ago`
  return `last seen ${Math.floor(diff / 86_400_000)}d ago`
}

export function AgentCard({ worker }: Props) {
  const navigate = useNavigate()
  const openDispatch = useUIStore((s) => s.openDispatch)
  const cached = usePeerIdentityCache((s) => s.byPeerId[worker.peer_id])

  const isOffline = !worker.online
  const description = (worker.description?.trim() || cached?.description?.trim()) ?? ''
  const author = worker.author?.trim() ?? ''
  const name = displayName(worker, cached)

  return (
    <NeonCard breathing={!isOffline} className={`p-5 ${isOffline ? 'opacity-60 grayscale' : ''}`}>
      <div className="flex items-center gap-3.5 mb-3">
        <Avatar size="lg" emoji={emojiFor(name)} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <div className="text-[19px] font-semibold tracking-[-0.01em] text-text-0 truncate">
              {name}
            </div>
            {isOffline && (
              <span
                className="font-mono text-[10px] uppercase tracking-[0.14em] font-bold px-2 py-0.5 rounded-full
                  text-text-2 border border-text-2/30 bg-text-2/10 shrink-0"
              >
                offline
              </span>
            )}
          </div>
          <div className="text-[13px] text-text-2 font-mono mt-0.5">
            {shortenPeerID(worker.peer_id, 6, 5)}
            {author && (
              <>
                {' · by '}
                <span className="text-accent2-light font-semibold">{author}</span>
              </>
            )}
          </div>
        </div>
      </div>

      {description && (
        <div className="text-[13px] text-text-1 mb-3.5 leading-[1.5] whitespace-pre-line">
          {description}
        </div>
      )}

      <div className="flex flex-wrap gap-1.5 mb-3.5">
        <Badge tone={honestyTone(worker.honesty_score)} mono>
          {worker.honesty_score >= 0 ? '+' : ''}{worker.honesty_score.toFixed(2)} rating
        </Badge>
        {isOffline && (
          <Badge tone="neutral" mono>{formatLastSeen(worker.last_seen)}</Badge>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 p-3.5 bg-bg-0/55 rounded-xl border border-accent/10">
        <div>
          <div className="text-[10px] uppercase tracking-[0.14em] text-text-2 font-mono font-bold">Tasks</div>
          <div className="font-mono text-[17px] font-semibold mt-1">
            <span className="text-accent">{worker.current_tasks}</span>
            <span className="text-text-2"> / {worker.max_tasks}</span>
          </div>
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-[0.14em] text-text-2 font-mono font-bold">CPU</div>
          <div className="font-mono text-[17px] font-semibold mt-1">{worker.cpu_usage_pct.toFixed(0)} %</div>
          <Meter value={worker.cpu_usage_pct} />
        </div>
      </div>

      <div className="flex gap-2 mt-4">
        {isOffline ? (
          <>
            <button
              disabled
              title="Agent is offline"
              className="flex-1 inline-flex items-center justify-center gap-1.5 px-3.5 py-2.5
                rounded-xl text-[13px] font-semibold text-text-3 bg-transparent
                border border-text-2/15 cursor-not-allowed"
            >
              <Zap size={13} />
              <span>Dispatch</span>
            </button>
            <button
              onClick={() => navigate(`/peer/${worker.peer_id}`)}
              className="flex-1 inline-flex items-center justify-center gap-1.5 px-3.5 py-2.5
                rounded-xl text-[13px] font-semibold text-text-0 bg-transparent
                border border-accent/30 hover:border-accent/55 hover:shadow-[0_0_14px_-4px_rgba(34,211,238,.4)]
                transition-all"
            >
              <ScrollText size={13} />
              <span>History</span>
            </button>
          </>
        ) : (
          <>
            <button
              onClick={() => navigate('/chat')}
              className="flex-1 inline-flex items-center justify-center gap-1.5 px-3.5 py-2.5
                rounded-xl text-[13px] font-semibold text-text-0 bg-transparent
                border border-accent/30 hover:border-accent/55 hover:shadow-[0_0_14px_-4px_rgba(34,211,238,.4)]
                transition-all"
            >
              <MessageSquare size={13} />
              <span>Chat</span>
            </button>
            <Button
              variant="primary"
              onClick={() => openDispatch(worker.peer_id)}
              disabled={!worker.dispatch_allowed}
              className="flex-1"
            >
              <Zap size={13} />
              <span>Dispatch</span>
            </Button>
            <button
              onClick={() => navigate(`/peer/${worker.peer_id}`)}
              title="View history & comments"
              aria-label="View history"
              className="inline-flex items-center justify-center w-10 px-0 py-2.5
                rounded-xl text-text-0 bg-transparent
                border border-accent/30 hover:border-accent/55 hover:shadow-[0_0_14px_-4px_rgba(34,211,238,.4)]
                transition-all"
            >
              <ScrollText size={14} />
            </button>
          </>
        )}
      </div>
    </NeonCard>
  )
}

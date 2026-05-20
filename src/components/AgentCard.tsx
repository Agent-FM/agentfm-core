import { Zap, MessageSquare } from 'lucide-react'
import type { WorkerProfile } from '../types/api'
import { NeonCard } from './primitives/NeonCard'
import { Avatar } from './primitives/Avatar'
import { Meter } from './primitives/Meter'
import { GradientButton } from './primitives/GradientButton'
import { Badge } from './primitives/Badge'
import { shortenPeerID } from '../lib/peer'
import { displayName } from '../lib/displayName'
import { useUIStore } from '../lib/store'
import { useNavigate } from 'react-router-dom'

interface Props {
  worker: WorkerProfile
  onHistory?: () => void
  onDispatch?: () => void
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

export function AgentCard({ worker }: Props) {
  const navigate = useNavigate()
  const openDispatch = useUIStore((s) => s.openDispatch)
  return (
    <NeonCard breathing className="p-5">
      <div className="flex items-center gap-3.5 mb-4">
        <Avatar size="lg" emoji={emojiFor(worker.name)} />
        <div className="min-w-0">
          <div className="text-[19px] font-semibold tracking-[-0.01em] text-text-0 truncate">
            {displayName(worker)}
          </div>
          <div className="text-[13px] text-text-2 font-mono mt-0.5">
            {shortenPeerID(worker.peer_id, 6, 5)}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-1.5 mb-3.5">
        <Badge tone={honestyTone(worker.honesty_score)} mono>
          {worker.honesty_score >= 0 ? '+' : ''}{worker.honesty_score.toFixed(2)} honesty
        </Badge>
        <Badge tone={worker.dispatch_allowed ? 'cyan' : 'rose'} mono>
          {worker.dispatch_allowed ? '✓ dispatch' : '✗ refused'}
        </Badge>
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
        <GradientButton
          onClick={() => openDispatch(worker.peer_id)}
          disabled={!worker.dispatch_allowed}
          className="flex-1"
        >
          <Zap size={13} />
          <span>Dispatch</span>
        </GradientButton>
      </div>
    </NeonCard>
  )
}

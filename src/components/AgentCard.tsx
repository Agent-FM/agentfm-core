import { motion } from 'framer-motion'
import { ArrowRight, History } from 'lucide-react'
import { shortenPeerID, shortenDigest } from '../lib/peer'
import { displayName } from '../lib/displayName'
import { Button } from './primitives/Button'
import { Badge } from './primitives/Badge'
import { StatusDot } from './primitives/StatusDot'
import { lift } from '../lib/motion'
import type { WorkerProfile } from '../types/api'

interface Props {
  worker: WorkerProfile
  onHistory: () => void
  onDispatch: () => void
}

export function AgentCard({ worker, onHistory, onDispatch }: Props) {
  const busy = worker.online && worker.current_tasks >= worker.max_tasks
  const offline = !worker.online
  const equivocator = worker.is_equivocator
  const canDispatch = worker.dispatch_allowed && !busy && worker.online

  const dotTone: 'cyan' | 'amber' | 'rose' | 'neutral' =
    equivocator ? 'rose' : busy ? 'amber' : worker.online ? 'cyan' : 'neutral'

  const stripVisible = worker.online && !equivocator

  return (
    <motion.div
      whileHover={offline ? undefined : lift.whileHover}
      transition={lift.transition}
      className={`relative bg-bg-1 border border-border-0 rounded-xl pl-5 pr-4 py-4 grid grid-cols-[1fr_auto] gap-4 items-center transition-all overflow-hidden ${
        offline ? 'opacity-60' : 'hover:border-accent/40 hover:shadow-[0_10px_30px_-14px_rgba(34,211,238,.35)]'
      }`}
    >
      {stripVisible && (
        <span className="absolute left-0 top-0 bottom-0 w-1.5 bg-gradient-to-b from-accent to-accent2" />
      )}

      <div>
        <div className="flex items-center gap-2 flex-wrap">
          <StatusDot tone={dotTone} pulse={worker.online && !busy && !equivocator} />
          <h4 className="text-base font-semibold text-text-0">{displayName(worker)}</h4>
          {worker.agent_capability && <Badge tone="violet" mono>{worker.agent_capability}</Badge>}
          {busy && <Badge tone="amber">busy {worker.current_tasks}/{worker.max_tasks}</Badge>}
        </div>
        <div className="text-2xs text-text-2 font-mono mt-1.5">
          {shortenPeerID(worker.peer_id, 12, 5)}
          {worker.model && <> · {worker.model}</>}
          {worker.agent_image_digest && <> · {shortenDigest(worker.agent_image_digest, 8)}</>}
        </div>
        <div className="flex flex-wrap gap-1.5 items-center mt-2">
          {equivocator ? (
            <Badge tone="rose">⚠ equivocator</Badge>
          ) : (
            <Badge tone={worker.honesty_score > 0.3 ? 'lime' : worker.honesty_score < -0.5 ? 'rose' : 'neutral'} mono>
              {worker.honesty_score >= 0 ? '+' : ''}{worker.honesty_score.toFixed(2)}
            </Badge>
          )}
          {!equivocator && (
            <Badge tone={worker.dispatch_allowed ? 'lime' : 'rose'}>
              {worker.dispatch_allowed ? '✓ allowed' : '✗ refused'}
            </Badge>
          )}
          {worker.online && (
            <span className="text-2xs text-text-2 ml-1">
              {worker.cpu_usage_pct.toFixed(0)}% cpu · {worker.ram_free_gb.toFixed(1)} GB free
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-1.5">
        <Button onClick={onHistory}>
          <History size={12} />
          <span>History</span>
        </Button>
        <Button variant="primary" onClick={onDispatch} disabled={!canDispatch}>
          <span>{equivocator ? 'Refused' : busy ? 'At capacity' : 'Dispatch'}</span>
          <ArrowRight size={12} />
        </Button>
      </div>
    </motion.div>
  )
}

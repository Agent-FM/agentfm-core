import { motion } from 'framer-motion';
import { shortenPeerID, shortenDigest } from '../lib/peer';
import { Button } from './primitives/Button';
import { HonestyBadge } from './HonestyBadge';
import { DispatchBadge } from './DispatchBadge';
import { EquivocatorBadge } from './EquivocatorBadge';
import { CapabilityBadge } from './CapabilityBadge';
import { displayName } from '../lib/displayName';
import type { WorkerProfile } from '../types/api';

interface Props {
  worker: WorkerProfile;
  onHistory: () => void;
  onDispatch: () => void;
}

export function AgentCard({ worker, onHistory, onDispatch }: Props) {
  const busy = worker.online && worker.current_tasks >= worker.max_tasks;
  const offline = !worker.online;
  const equivocator = worker.is_equivocator;
  const canDispatch = worker.dispatch_allowed && !busy && worker.online;

  let dotClass = 'bg-text-3';
  if (worker.online) {
    dotClass = busy
      ? 'bg-amber-500 shadow-[0_0_6px_#f59e0b]'
      : 'bg-accent shadow-[0_0_8px_var(--accent)] animate-pulse';
  } else if (equivocator) {
    dotClass = 'bg-rose-500 shadow-[0_0_6px_#ef4444]';
  }

  return (
    <motion.div
      whileHover={{ y: -2, boxShadow: '0 8px 24px -12px rgba(0,0,0,0.5)' }}
      transition={{ duration: 0.18 }}
      className={`bg-bg-1 border border-border-0 hover:border-accent/40 rounded-xl px-4 py-3.5 grid grid-cols-[auto_1fr_auto] gap-4 items-center transition-colors ${offline ? 'opacity-65' : ''}`}
    >
      <span className={`w-2 h-2 rounded-full ${dotClass}`} />

      <div>
        <div className="flex items-center gap-2.5 flex-wrap">
          <h4 className="text-sm font-semibold text-text-0">{displayName(worker)}</h4>
          {worker.agent_capability && <CapabilityBadge name={worker.agent_capability} />}
          {busy && (
            <span className="text-[10px] text-amber-400 uppercase tracking-wider">
              ⏳ busy {worker.current_tasks}/{worker.max_tasks}
            </span>
          )}
        </div>
        <div className="text-[11px] text-text-2 font-mono mt-1">
          {shortenPeerID(worker.peer_id, 12, 5)}
          {worker.agent_image_ref && (
            <> · {worker.model || 'unknown'}</>
          )}
          {worker.agent_image_digest && (
            <> · {shortenDigest(worker.agent_image_digest, 8)}</>
          )}
        </div>
        <div className="flex flex-wrap gap-1.5 items-center mt-1.5">
          {equivocator ? (
            <EquivocatorBadge />
          ) : (
            <HonestyBadge score={worker.honesty_score} />
          )}
          {!equivocator && (
            <DispatchBadge allowed={worker.dispatch_allowed} reason={worker.dispatch_refuse_reason} />
          )}
          {worker.online && !offline && (
            <span className="text-[11px] text-text-2 ml-1">
              {worker.cpu_usage_pct.toFixed(0)}% cpu · {worker.ram_free_gb.toFixed(1)} GB free
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-1.5">
        <Button onClick={onHistory}>History</Button>
        <Button variant="primary" onClick={onDispatch} disabled={!canDispatch}>
          {equivocator ? 'Refused' : busy ? 'At capacity' : 'Dispatch ↵'}
        </Button>
      </div>
    </motion.div>
  );
}

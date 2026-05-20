import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown } from 'lucide-react'
import { useWorkers } from '../../lib/query';
import { shortenPeerID } from '../../lib/peer';
import { displayName } from '../../lib/displayName';

interface Props {
  pinnedPeerId: string | null;
  preferredModel: string;
  onPin: (peerId: string | null) => void;
}

export function AgentPicker({ pinnedPeerId, preferredModel, onPin }: Props) {
  const [open, setOpen] = useState(false);
  const { data } = useWorkers(false);
  const online = data?.agents ?? [];

  const pinnedWorker = online.find((w) => w.peer_id === pinnedPeerId);
  const label = pinnedWorker
    ? `pinned: ${displayName(pinnedWorker)}`
    : preferredModel && preferredModel !== 'auto'
      ? `${preferredModel} (auto-route)`
      : 'Auto-route';

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="inline-flex items-center gap-2 bg-bg-2 border border-border-0 px-2.5 py-1.5 rounded-md text-xs text-text-1 hover:text-text-0"
      >
        <span className="w-1.5 h-1.5 rounded-full bg-accent shadow-[0_0_6px_#22d3ee] animate-pulse-cyan" />
        <span>{label}</span>
        <ChevronDown size={12} className="text-text-2" />
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.12 }}
            className="absolute top-full mt-1 left-0 bg-bg-1 border border-border-0 rounded-md shadow-xl w-72 max-h-72 overflow-auto z-20 neon-glow-cyan"
            onMouseLeave={() => setOpen(false)}
          >
            <button
              onClick={() => {
                onPin(null);
                setOpen(false);
              }}
              className={`w-full text-left px-3 py-2 text-xs hover:bg-bg-2 border-b border-border-0 ${
                pinnedPeerId === null ? 'text-accent' : 'text-text-1'
              }`}
            >
              <div className="font-medium">Auto-route</div>
              <div className="text-text-2 text-[11px]">
                Mesh picks the least-loaded compatible worker
                {preferredModel && preferredModel !== 'auto' ? ` matching "${preferredModel}"` : ''}
              </div>
            </button>
            {online.map((w) => {
              const isPinned = pinnedPeerId === w.peer_id;
              return (
                <button
                  key={w.peer_id}
                  onClick={() => {
                    onPin(w.peer_id);
                    setOpen(false);
                  }}
                  className={`relative w-full text-left px-3 py-2 text-xs hover:bg-bg-2 ${
                    isPinned ? 'text-accent bg-accent/10' : 'text-text-1'
                  }`}
                >
                  {isPinned && <span className="absolute left-0 top-0 bottom-0 w-[2px] bg-accent" />}
                  <div className="font-medium">{displayName(w)}</div>
                  <div className="text-accent2-light text-[11px] font-mono">
                    {shortenPeerID(w.peer_id, 12, 5)} · honesty{' '}
                    {w.honesty_score.toFixed(2)}
                  </div>
                </button>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

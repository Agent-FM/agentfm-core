import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown } from 'lucide-react'
import { useWorkers } from '../../lib/query';
import { shortenPeerID } from '../../lib/peer';
import { displayName } from '../../lib/displayName';

interface Props {
  pinnedPeerId: string | null;
  onPin: (peerId: string | null) => void;
}

export function AgentPicker({ pinnedPeerId, onPin }: Props) {
  const [open, setOpen] = useState(false);
  const { data } = useWorkers(false);
  const online = data?.agents ?? [];

  const pinnedWorker = online.find((w) => w.peer_id === pinnedPeerId);
  const label = pinnedWorker ? `pinned: ${displayName(pinnedWorker)}` : 'Pick an agent…';
  const isUnpinned = !pinnedWorker;

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={`inline-flex items-center gap-2 border px-2.5 py-1.5 rounded-md text-xs hover:text-text-0 ${
          isUnpinned
            ? 'bg-warn/10 border-warn/40 text-warn'
            : 'bg-bg-2 border-border-0 text-text-1'
        }`}
      >
        <span
          className={`w-1.5 h-1.5 rounded-full animate-pulse-cyan ${
            isUnpinned
              ? 'bg-warn shadow-[0_0_6px_#facc15]'
              : 'bg-accent shadow-[0_0_6px_#22d3ee]'
          }`}
        />
        <span>{label}</span>
        <ChevronDown size={12} className={isUnpinned ? 'text-warn' : 'text-text-2'} />
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
            {online.length === 0 && (
              <div className="px-3 py-3 text-xs text-text-2">
                No online agents in this project. Wait for one to appear on the Radar.
              </div>
            )}
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
                    {shortenPeerID(w.peer_id, 12, 5)} · rating{' '}
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

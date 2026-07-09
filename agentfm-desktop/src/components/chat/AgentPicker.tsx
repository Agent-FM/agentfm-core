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
        className={`inline-flex items-center gap-2 border h-[22px] px-2.5 rounded-ctl text-xs transition-colors duration-150 hover:text-text-0 ${
          isUnpinned
            ? 'bg-warn/10 border-warn/40 text-warn'
            : 'bg-[#3A3A3E] border-border-0 text-text-1'
        }`}
      >
        <span
          className={`w-1.5 h-1.5 rounded-full ${
            isUnpinned ? 'bg-warn' : 'bg-accent'
          }`}
        />
        <span>{label}</span>
        <ChevronDown size={12} strokeWidth={1.5} className={isUnpinned ? 'text-warn' : 'text-text-2'} />
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="absolute top-full mt-1 left-0 glass-strong rounded-card shadow-float w-72 max-h-72 overflow-auto z-20"
            onMouseLeave={() => setOpen(false)}
          >
            {online.length === 0 && (
              <div className="px-2.5 py-2 text-xs text-text-2">
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
                  className={`relative w-full text-left px-2.5 py-1.5 text-xs border-b border-border-0 last:border-b-0 transition-colors duration-150 hover:bg-white/[0.04] ${
                    isPinned ? 'row-selected text-text-0' : 'text-text-1'
                  }`}
                >
                  <div className="font-medium">{displayName(w)}</div>
                  <div className="text-text-2 text-2xs font-mono tabular-nums">
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

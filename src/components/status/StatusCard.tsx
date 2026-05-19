import { motion } from 'framer-motion';
import { ReactNode } from 'react';

interface Props {
  title: string;
  status?: 'ok' | 'warn' | 'err' | 'idle';
  children: ReactNode;
}

export function StatusCard({ title, status = 'ok', children }: Props) {
  const dotColor = {
    ok: 'bg-accent',
    warn: 'bg-amber-500',
    err: 'bg-rose-500',
    idle: 'bg-text-3',
  }[status];

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.18 }}
      className="bg-bg-1 border border-border-0 rounded-lg p-4"
    >
      <div className="text-[11px] uppercase tracking-wider text-text-2 mb-2 flex items-center gap-1.5">
        <span className={`w-1.5 h-1.5 rounded-full ${dotColor}`} />
        {title}
      </div>
      {children}
    </motion.div>
  );
}

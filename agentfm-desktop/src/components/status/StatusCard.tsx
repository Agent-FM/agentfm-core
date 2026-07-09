import { ReactNode } from 'react';

interface Props {
  title: string;
  status?: 'ok' | 'warn' | 'err' | 'idle';
  children: ReactNode;
}

export function StatusCard({ title, status = 'ok', children }: Props) {
  const dotColor = {
    ok: 'bg-ok',
    warn: 'bg-warn',
    err: 'bg-bad',
    idle: 'bg-text-3',
  }[status];

  return (
    <div className="glass rounded-card p-3">
      <div className="text-2xs font-medium uppercase tracking-[0.06em] text-text-1 mb-1.5 flex items-center gap-1.5">
        <span className={`w-1.5 h-1.5 rounded-full ${dotColor}`} />
        {title}
      </div>
      {children}
    </div>
  );
}

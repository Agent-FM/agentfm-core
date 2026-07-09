import { ReactNode } from 'react'

export type BadgeTone = 'accent' | 'ok' | 'warn' | 'bad' | 'neutral'

interface Props {
  tone?: BadgeTone
  mono?: boolean
  children: ReactNode
  className?: string
  title?: string
}

const TONES: Record<BadgeTone, string> = {
  accent:  'bg-accent/15 border-accent/35 text-accent',
  ok:      'bg-ok/15 border-ok/40 text-ok',
  warn:    'bg-warn/15 border-warn/40 text-warn',
  bad:     'bg-bad/15 border-bad/40 text-bad',
  neutral: 'bg-raised border-border-1 text-text-1',
}

export function Badge({ tone = 'accent', mono = false, children, className, title }: Props) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 px-1.5 py-px rounded-[4px] text-2xs border ${TONES[tone]} ${mono ? 'font-mono tabular-nums' : ''} ${className ?? ''}`}
    >
      {children}
    </span>
  )
}

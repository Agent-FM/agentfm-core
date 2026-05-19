import { ReactNode } from 'react'

export type BadgeTone = 'cyan' | 'violet' | 'lime' | 'amber' | 'rose' | 'neutral'

interface Props {
  tone?: BadgeTone
  mono?: boolean
  children: ReactNode
  className?: string
  title?: string
}

const TONES: Record<BadgeTone, string> = {
  cyan:    'bg-accent/15 border-accent/35 text-accent',
  violet:  'bg-accent2/15 border-accent2/40 text-accent2-light',
  lime:    'bg-ok/15 border-ok/40 text-ok',
  amber:   'bg-warn/15 border-warn/40 text-warn',
  rose:    'bg-bad/15 border-bad/40 text-bad',
  neutral: 'bg-bg-2 border-border-0 text-text-2',
}

export function Badge({ tone = 'cyan', mono = false, children, className, title }: Props) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-2xs border ${TONES[tone]} ${mono ? 'font-mono' : ''} ${className ?? ''}`}
    >
      {children}
    </span>
  )
}

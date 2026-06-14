import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  tone?: 'cyan' | 'rose'
}

export function SectionLabel({ children, tone }: Props) {
  const toneColor = tone === 'rose' ? 'text-bad' : tone === 'cyan' ? 'text-accent' : 'text-text-2'
  return (
    <div className={`text-2xs font-medium uppercase tracking-[0.14em] ${toneColor}`}>{children}</div>
  )
}

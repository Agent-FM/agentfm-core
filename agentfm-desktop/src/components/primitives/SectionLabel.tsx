import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  tone?: 'cyan' | 'rose'
}

export function SectionLabel({ children }: Props) {
  return (
    <div className="text-2xs font-medium uppercase tracking-[0.14em] text-text-2">{children}</div>
  )
}

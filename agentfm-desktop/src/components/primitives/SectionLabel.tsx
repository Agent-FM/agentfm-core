import type { ElementType, ReactNode } from 'react'

interface Props {
  children: ReactNode
  tone?: 'accent' | 'bad'
  as?: ElementType
}

export function SectionLabel({ children, tone, as: Tag = 'div' }: Props) {
  const toneColor = tone === 'bad' ? 'text-bad' : tone === 'accent' ? 'text-accent' : 'text-text-1'
  return (
    <Tag className={`text-2xs font-medium ${toneColor}`}>{children}</Tag>
  )
}

import type { ReactNode } from 'react'

interface Props {
  size?: 'sm' | 'md' | 'lg'
  emoji?: string
  children?: ReactNode
}

const SIZES = {
  sm: { box: 26, font: 13 },
  md: { box: 48, font: 22 },
  lg: { box: 54, font: 24 },
}

export function Avatar({ size = 'md', emoji, children }: Props) {
  const s = SIZES[size]
  return (
    <div
      className="relative shrink-0 flex items-center justify-center rounded-ctl bg-bg-well border border-border-0 text-text-1"
      style={{ width: s.box, height: s.box, fontSize: s.font }}
    >
      {emoji ?? children}
    </div>
  )
}

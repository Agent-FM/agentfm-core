import type { ReactNode } from 'react'

interface Props {
  size?: 'sm' | 'md' | 'lg'
  emoji?: string
  children?: ReactNode
}

const SIZES = {
  sm: { box: 26, font: 13, radius: 8 },
  md: { box: 48, font: 22, radius: 12 },
  lg: { box: 54, font: 24, radius: 13 },
}

export function Avatar({ size = 'md', emoji, children }: Props) {
  const s = SIZES[size]
  return (
    <div
      className="relative shrink-0 flex items-center justify-center bg-bg-1 border border-border-0 text-text-1"
      style={{ width: s.box, height: s.box, borderRadius: s.radius, fontSize: s.font }}
    >
      {emoji ?? children}
    </div>
  )
}

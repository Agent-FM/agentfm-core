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
      className="relative shrink-0"
      style={{ width: s.box, height: s.box }}
    >
      <span
        className="absolute animate-halo-rotate"
        style={{
          inset: -2,
          borderRadius: s.radius + 2,
          background: 'conic-gradient(from 0deg, #22d3ee, #a855f7, #22d3ee)',
          opacity: 0.55,
        }}
      />
      <div
        className="relative flex items-center justify-center bg-bg-1"
        style={{
          width: s.box,
          height: s.box,
          borderRadius: s.radius,
          fontSize: s.font,
          background:
            'linear-gradient(135deg, rgba(34,211,238,.25), rgba(168,85,247,.25)), #0d1117',
        }}
      >
        {emoji ?? children}
      </div>
    </div>
  )
}

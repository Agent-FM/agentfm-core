import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  tone?: 'cyan' | 'rose'
}

export function SectionLabel({ children, tone = 'cyan' }: Props) {
  const color = tone === 'rose' ? '#f87171' : '#22d3ee'
  const glow = tone === 'rose' ? 'rgba(244,63,94,.7)' : 'rgba(34,211,238,.7)'
  return (
    <div
      className="inline-flex items-center gap-2 mb-2.5 font-mono font-bold uppercase"
      style={{ color, fontSize: '11px', letterSpacing: '0.18em' }}
    >
      <span style={{ textShadow: `0 0 8px ${glow}` }}>▌</span>
      {children}
    </div>
  )
}

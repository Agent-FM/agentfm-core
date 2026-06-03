import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  breathing?: boolean
  className?: string
}

export function NeonCard({ children, breathing = false, className = '' }: Props) {
  const baseShadow = breathing
    ? 'animate-glow-cycle'
    : 'border border-accent/25'
  return (
    <div
      className={`relative overflow-hidden rounded-2xl
        bg-gradient-to-br from-bg-1/[.92] to-bg-0/[.96]
        ${baseShadow} ${className}`}
    >
      <span className="absolute inset-x-0 top-0 h-px overflow-hidden">
        <span
          className="absolute top-0 left-0 h-px w-3/5 animate-top-sweep"
          style={{
            background: 'linear-gradient(90deg, transparent, #22d3ee, transparent)',
            boxShadow: '0 0 10px #22d3ee',
          }}
        />
      </span>
      {children}
    </div>
  )
}

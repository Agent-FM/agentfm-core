import { HTMLAttributes, ReactNode } from 'react'

type Density = 'default' | 'compact' | 'spacious'

interface Props extends HTMLAttributes<HTMLDivElement> {
  density?: Density
  live?: boolean
  children: ReactNode
}

const PADDING: Record<Density, string> = {
  default: 'p-4',
  compact: 'p-3',
  spacious: 'p-6',
}

export function Card({ density = 'default', live = false, children, className, ...rest }: Props) {
  return (
    <div
      className={`relative bg-bg-1 border border-border-0 rounded-xl transition-all ${PADDING[density]} ${live ? 'neon-glow-cyan' : 'hover:border-accent/30'} ${className ?? ''}`}
      {...rest}
    >
      {live && (
        <span className="absolute -top-px -left-px w-2 h-2 rounded-full bg-accent shadow-[0_0_8px_#22d3ee] animate-pulse-cyan" />
      )}
      {children}
    </div>
  )
}

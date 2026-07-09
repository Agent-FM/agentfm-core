import { HTMLAttributes, ReactNode } from 'react'

type Density = 'default' | 'compact' | 'spacious'

interface Props extends HTMLAttributes<HTMLDivElement> {
  density?: Density
  live?: boolean
  children: ReactNode
}

const PADDING: Record<Density, string> = {
  default: 'p-3',
  compact: 'p-2',
  spacious: 'p-4',
}

export function Card({ density = 'default', live = false, children, className, ...rest }: Props) {
  return (
    <div
      className={`relative rounded-card transition-colors duration-150 ${PADDING[density]} ${
        live ? 'glass-live' : 'glass glass-hover'
      } ${className ?? ''}`}
      {...rest}
    >
      {live && (
        <span data-live-dot className="absolute top-3 right-3 w-2 h-2 rounded-full bg-ok animate-pulse-cyan" />
      )}
      {children}
    </div>
  )
}

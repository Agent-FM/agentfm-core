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
      className={`relative rounded-[14px] bg-bg-2 border transition-all duration-200 ${PADDING[density]} ${
        live
          ? 'border-accent/35 shadow-[0_10px_34px_-16px_rgba(247,147,30,.4)]'
          : 'border-border-0 shadow-card hover:border-border-1 hover:shadow-card-hover hover:-translate-y-0.5'
      } ${className ?? ''}`}
      {...rest}
    >
      {live && (
        <span data-live-dot className="absolute top-3 right-3 w-2 h-2 rounded-full bg-accent shadow-[0_0_8px_rgba(247,147,30,.85)] animate-pulse-cyan" />
      )}
      {children}
    </div>
  )
}

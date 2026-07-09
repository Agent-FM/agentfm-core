import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  className?: string
}

export function RoutePage({ children, className = '' }: Props) {
  return (
    <div className={`relative min-h-full h-full ${className}`}>
      <div className="relative h-full overflow-auto">{children}</div>
    </div>
  )
}

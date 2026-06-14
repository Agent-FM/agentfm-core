import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  className?: string
}

export function RoutePage({ children, className = '' }: Props) {
  return (
    <div className={`relative overflow-hidden min-h-full h-full ${className}`}>
      <div
        className="absolute -top-1/4 -left-1/4 w-[70%] h-[70%] pointer-events-none animate-aurora rounded-full blur-3xl"
        style={{ background: 'radial-gradient(circle, rgba(34,211,238,.10), transparent 60%)' }}
      />
      <div
        className="absolute -bottom-1/3 -right-1/4 w-[70%] h-[70%] pointer-events-none animate-aurora rounded-full blur-3xl"
        style={{ background: 'radial-gradient(circle, rgba(255,255,255,.05), transparent 60%)', animationDelay: '-9s' }}
      />
      <div className="relative z-10 h-full overflow-auto">{children}</div>
    </div>
  )
}

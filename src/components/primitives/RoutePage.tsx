import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  className?: string
}

export function RoutePage({ children, className = '' }: Props) {
  return (
    <div className={`relative overflow-hidden min-h-full h-full ${className}`}>
      <div
        className="route-page__blobs absolute inset-0 pointer-events-none animate-drift-bg"
        style={{
          background:
            'radial-gradient(circle at 15% 0%, rgba(34,211,238,.12), transparent 55%),' +
            'radial-gradient(circle at 85% 100%, rgba(168,85,247,.14), transparent 55%)',
        }}
      />
      <div
        className="route-page__grid absolute inset-0 pointer-events-none animate-mesh-grid opacity-80"
        style={{
          backgroundImage:
            'linear-gradient(rgba(34,211,238,.06) 1px, transparent 1px),' +
            'linear-gradient(90deg, rgba(34,211,238,.06) 1px, transparent 1px)',
          backgroundSize: '60px 60px',
        }}
      />
      <div className="relative z-10 h-full overflow-auto">{children}</div>
    </div>
  )
}

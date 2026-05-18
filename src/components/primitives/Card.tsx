export function Card({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`bg-bg-1 border border-border-0 rounded-lg ${className}`}>{children}</div>
  )
}

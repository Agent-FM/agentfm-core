interface BoxProps {
  className?: string
}

export function SkeletonBox({ className }: BoxProps) {
  return (
    <div className={`relative overflow-hidden bg-bg-2 rounded ${className ?? ''}`}>
      <div className="absolute inset-0 animate-shimmer bg-gradient-to-r from-transparent via-accent/10 to-transparent" />
    </div>
  )
}

interface RowProps {
  delay?: number
  className?: string
}

export function SkeletonRow({ delay = 0, className }: RowProps) {
  return (
    <div
      className={`bg-bg-1 border border-border-0 rounded-xl p-4 flex items-center gap-3 ${className ?? ''}`}
      style={{ animationDelay: `${delay}ms` }}
    >
      <SkeletonBox className="w-2 h-2" />
      <div className="flex-1 space-y-2">
        <SkeletonBox className="h-3.5 w-40" />
        <SkeletonBox className="h-2.5 w-64" />
      </div>
      <SkeletonBox className="h-7 w-24" />
    </div>
  )
}

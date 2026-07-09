interface BoxProps {
  className?: string
}

export function SkeletonBox({ className }: BoxProps) {
  return (
    <div className={`bg-white/[0.06] rounded animate-pulse ${className ?? ''}`} />
  )
}

interface RowProps {
  delay?: number
  className?: string
}

export function SkeletonRow({ delay = 0, className }: RowProps) {
  return (
    <div
      className={`h-6 px-2 border-b border-border-0 flex items-center gap-3 ${className ?? ''}`}
      style={{ animationDelay: `${delay}ms` }}
    >
      <SkeletonBox className="w-2 h-2" />
      <SkeletonBox className="h-2.5 w-40" />
      <SkeletonBox className="h-2.5 w-64 flex-1 max-w-[16rem]" />
      <SkeletonBox className="h-2.5 w-16 ml-auto" />
    </div>
  )
}

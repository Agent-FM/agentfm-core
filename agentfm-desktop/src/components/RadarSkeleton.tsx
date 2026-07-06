import { SkeletonRow } from './primitives/Skeleton'
import { Badge } from './primitives/Badge'

export function RadarSkeleton() {
  return (
    <div className="p-7">
      <div className="flex items-baseline gap-3 mb-1.5">
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Agent Radar</h1>
        <Badge tone="cyan"><span className="animate-pulse-cyan inline-block w-1 h-1 rounded-full bg-accent mr-1" />LISTENING</Badge>
      </div>
      <p className="text-text-2 mb-6">Waiting for the first telemetry beacon…</p>
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        {[0, 1, 2].map((i) => <SkeletonRow key={i} delay={i * 120} />)}
      </div>
    </div>
  )
}

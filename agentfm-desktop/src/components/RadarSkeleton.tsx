import { Badge } from './primitives/Badge'

export function RadarSkeleton() {
  return (
    <div>
      <div className="px-4 pt-4 pb-3">
        <div className="flex items-baseline gap-3">
          <h1 className="text-lg font-semibold text-text-0">Agent Radar</h1>
          <Badge tone="accent"><span className="animate-pulse-cyan inline-block w-1 h-1 rounded-full bg-accent mr-1" />LISTENING</Badge>
        </div>
        <p className="text-text-2 text-2xs mt-0.5">Waiting for the first telemetry beacon…</p>
      </div>
      <div className="border-t border-border-0">
        {[0, 1, 2].map((i) => (
          <div
            key={i}
            className="px-3 py-2 border-b border-border-0 flex items-center gap-3 animate-pulse"
            style={{ animationDelay: `${i * 120}ms` }}
          >
            <div className="w-2 h-2 rounded-full bg-white/[0.06] shrink-0" />
            <div className="flex-1 space-y-1.5">
              <div className="h-3 w-40 rounded bg-white/[0.06]" />
              <div className="h-2.5 w-64 rounded bg-white/[0.06]" />
            </div>
            <div className="h-[22px] w-24 rounded-ctl bg-white/[0.06]" />
          </div>
        ))}
      </div>
    </div>
  )
}

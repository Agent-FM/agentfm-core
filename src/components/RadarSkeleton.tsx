import { motion } from 'framer-motion';

export function RadarSkeleton() {
  return (
    <div className="p-7">
      <div className="flex items-baseline gap-3 mb-1">
        <h1 className="text-xl font-semibold text-text-0">Agent Radar</h1>
        <span className="inline-flex gap-1.5 items-center bg-accent-bg text-accent text-2xs uppercase tracking-wider px-2 py-0.5 rounded-full font-semibold">
          <span className="w-1 h-1 rounded-full bg-accent animate-pulse" />
          LISTENING
        </span>
      </div>
      <p className="text-text-2 mb-5">Waiting for the first telemetry beacon…</p>
      <div className="space-y-2.5">
        {[0, 1, 2].map((i) => (
          <SkeletonRow key={i} delay={i * 0.12} />
        ))}
      </div>
    </div>
  );
}

function SkeletonRow({ delay }: { delay: number }) {
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ delay }}
      className="bg-bg-1 border border-border-0 rounded-xl px-4 py-3.5 grid grid-cols-[auto_1fr_auto] gap-4 items-center"
    >
      <div className="w-2 h-2 rounded-full bg-bg-2 animate-pulse" />
      <div className="space-y-2">
        <div className="h-3.5 w-40 bg-bg-2 rounded animate-pulse" />
        <div className="h-2.5 w-64 bg-bg-2 rounded animate-pulse" />
        <div className="h-2.5 w-32 bg-bg-2 rounded animate-pulse" />
      </div>
      <div className="flex gap-1.5">
        <div className="h-7 w-16 bg-bg-2 rounded animate-pulse" />
        <div className="h-7 w-24 bg-bg-2 rounded animate-pulse" />
      </div>
    </motion.div>
  );
}

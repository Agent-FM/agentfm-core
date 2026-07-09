import type { ReactNode } from 'react'
import { Zap, ScrollText, Copy, Cpu, CircuitBoard, MemoryStick, Layers } from 'lucide-react'
import { toast } from 'sonner'
import type { WorkerProfile } from '../types/api'
import { Meter } from './primitives/Meter'
import { shortenPeerID } from '../lib/peer'
import { displayName } from '../lib/displayName'
import { useUIStore } from '../lib/store'
import { usePeerIdentityCache } from '../lib/peerIdentityCache'
import { useMetricsStore } from '../lib/metricsStore'
import { latestValue } from '../types/metrics'
import { useNavigate } from 'react-router-dom'
import { StarRow } from './primitives/StarRow'
import { starsFromScore } from '../lib/stars'

interface Props {
  worker: WorkerProfile
}

function trustClasses(score: number): string {
  if (score >= 0.3) return 'text-ok bg-ok/10'
  if (score <= -0.5) return 'text-bad bg-bad/10'
  return 'text-text-1 bg-white/[0.06]'
}

function formatLastSeen(iso: string | null): string {
  if (!iso) return 'never seen'
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return 'never seen'
  const diff = Date.now() - t
  if (diff < 60_000) return 'just now'
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return `${Math.floor(diff / 86_400_000)}d ago`
}

function Stat({
  icon: Icon,
  label,
  children,
  bar,
}: {
  icon: typeof Cpu
  label: string
  children: ReactNode
  bar?: number
}) {
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-1.5 text-2xs font-medium uppercase tracking-[0.05em] text-text-2 mb-1.5">
        <Icon size={12} strokeWidth={1.75} className="shrink-0" />
        {label}
      </div>
      <div className="font-mono text-[15px] font-semibold tabular-nums leading-none">{children}</div>
      {bar !== undefined && <Meter value={bar} />}
    </div>
  )
}

export function AgentCard({ worker }: Props) {
  const navigate = useNavigate()
  const openDispatch = useUIStore((s) => s.openDispatch)
  const cached = usePeerIdentityCache((s) => s.byPeerId[worker.peer_id])
  const peerBufs = useMetricsStore((s) => s.peerSeries.get(worker.peer_id))

  const isOffline = !worker.online
  const description = (worker.description?.trim() || cached?.description?.trim()) ?? ''
  const author = worker.author?.trim() ?? ''
  const name = displayName(worker, cached)

  const gpuBuf = peerBufs?.get('gpu')
  const gpuPct = gpuBuf ? latestValue(gpuBuf) : null
  const gpuBar = worker.has_gpu && gpuPct != null ? gpuPct : undefined
  const idle = worker.current_tasks === 0

  return (
    <div
      className={`glass rounded-card p-4 h-full flex flex-col gap-3.5 transition-[transform,border-color,box-shadow] duration-150 ease-out hover:-translate-y-[2px] hover:border-accent/30 hover:shadow-[0_10px_26px_-10px_rgba(0,0,0,0.65)] ${
        isOffline ? 'opacity-60 grayscale hover:border-border-1 hover:shadow-none' : ''
      }`}
    >
      {/* header */}
      <div className="flex items-center gap-3 min-w-0">
        <div
          className={`relative shrink-0 w-11 h-11 rounded-card flex items-center justify-center text-xl font-semibold select-none ${
            isOffline ? 'bg-white/[0.06] text-text-2' : 'bg-accent/15 text-accent ring-1 ring-inset ring-accent/25'
          }`}
          aria-hidden="true"
        >
          {name.charAt(0).toUpperCase()}
          <span
            className={`absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2 border-bg-0 ${
              isOffline ? 'bg-text-3' : 'bg-ok animate-pulse-cyan'
            }`}
          />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-lg font-semibold text-text-0 truncate">{name}</span>
            {isOffline && (
              <span className="text-2xs font-medium uppercase tracking-[0.06em] px-1.5 py-px rounded-full text-text-2 border border-border-1 shrink-0">
                offline {formatLastSeen(worker.last_seen)}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2 text-xs text-text-2 min-w-0 mt-0.5">
            <button
              onClick={() =>
                navigator.clipboard
                  .writeText(worker.peer_id)
                  .then(() => toast.success('Peer ID copied'))
                  .catch(() => toast.error('Copy failed'))
              }
              title={`Copy peer ID ${worker.peer_id}`}
              className="group/id inline-flex items-center gap-1 font-mono text-text-2 hover:text-text-0 transition-colors shrink-0 cursor-pointer"
            >
              {shortenPeerID(worker.peer_id, 6, 5)}
              <Copy size={11} className="opacity-0 group-hover/id:opacity-70 transition-opacity" />
            </button>
            {author && (
              <span className="truncate">by <span className="text-text-1 font-medium">{author}</span></span>
            )}
          </div>
        </div>
        <div
          className={`shrink-0 inline-flex items-center gap-1.5 rounded-full px-2 py-1 ${trustClasses(worker.honesty_score)}`}
          title={`Reputation ${worker.honesty_score >= 0 ? '+' : ''}${worker.honesty_score.toFixed(2)}`}
        >
          <StarRow value={starsFromScore(worker.honesty_score)} size={12} />
          <span className="font-mono text-2xs font-semibold tabular-nums">
            {worker.honesty_score >= 0 ? '+' : ''}{worker.honesty_score.toFixed(2)}
          </span>
        </div>
      </div>

      {/* description */}
      <p className="text-sm text-text-1 leading-relaxed line-clamp-2 min-h-[44px] bg-white/[0.03] rounded-ctl px-2.5 py-1.5">
        {description || <span className="italic text-text-2">No description provided</span>}
      </p>

      {/* tags */}
      {(worker.agent_capability || worker.model) && (
        <div className="flex flex-wrap gap-1.5 -mt-1">
          {worker.agent_capability && (
            <span className="font-mono text-2xs font-semibold text-accent bg-accent/12 border border-accent/25 rounded-ctl px-2 py-0.5 truncate max-w-full">
              {worker.agent_capability}
            </span>
          )}
          {worker.model && (
            <span className="font-mono text-2xs text-text-1 bg-white/[0.06] border border-border-0 rounded-ctl px-2 py-0.5 truncate max-w-full">
              {worker.model}
            </span>
          )}
        </div>
      )}

      {/* telemetry */}
      <div className="grid grid-cols-2 gap-x-5 gap-y-3.5 rounded-card bg-white/[0.025] px-3.5 py-3">
        <Stat icon={Cpu} label="CPU load" bar={worker.cpu_usage_pct}>
          <span className="text-text-0">{worker.cpu_usage_pct.toFixed(0)}</span>
          <span className="text-2xs text-text-2 ml-0.5">%</span>
        </Stat>
        <Stat icon={CircuitBoard} label="GPU" bar={gpuBar}>
          {worker.has_gpu ? (
            gpuPct != null ? (
              <>
                <span className="text-text-0">{Math.round(gpuPct)}</span>
                <span className="text-2xs text-text-2 ml-0.5">%</span>
              </>
            ) : (
              <span className="text-ok">Ready</span>
            )
          ) : (
            <span className="text-text-2">None</span>
          )}
        </Stat>
        <Stat icon={MemoryStick} label="RAM free">
          <span className="text-text-0">{worker.ram_free_gb.toFixed(1)}</span>
          <span className="text-2xs text-text-2 ml-0.5">GB</span>
        </Stat>
        <Stat icon={Layers} label="Task queue">
          <span className={idle ? 'text-ok' : 'text-accent'}>{worker.current_tasks}</span>
          <span className="text-text-2">/{worker.max_tasks}</span>
        </Stat>
      </div>

      {/* actions */}
      <div className="mt-auto flex items-stretch gap-2">
        <button
          onClick={() => !isOffline && openDispatch(worker.peer_id)}
          disabled={isOffline || !worker.dispatch_allowed}
          title={
            isOffline
              ? 'Agent is offline'
              : worker.dispatch_allowed
                ? 'Send a task to this agent'
                : worker.dispatch_refuse_reason ?? 'Dispatch refused'
          }
          className="flex-1 h-9 inline-flex items-center justify-center gap-1.5 rounded-ctl bg-accent text-accent-fg text-sm font-semibold shadow-[0_1px_2px_rgba(0,0,0,0.35)] hover:bg-accent-light active:scale-[0.98] disabled:opacity-40 disabled:pointer-events-none transition-[background-color,transform] duration-150 cursor-pointer"
        >
          <Zap size={15} strokeWidth={2} fill="currentColor" />
          Dispatch task
        </button>
        <button
          onClick={() => navigate(`/peer/${worker.peer_id}`)}
          title="View history and comments"
          className="h-9 px-3.5 inline-flex items-center justify-center gap-1.5 rounded-ctl border border-border-1 text-text-1 text-sm hover:text-text-0 hover:border-accent/50 hover:bg-white/[0.05] active:scale-[0.98] transition-[color,border-color,background-color,transform] duration-150 cursor-pointer"
        >
          <ScrollText size={15} strokeWidth={1.5} />
          History
        </button>
      </div>
    </div>
  )
}

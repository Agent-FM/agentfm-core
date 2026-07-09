import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { useWorkers } from '../lib/query'
import { usePeerIdentityCache, mergeWithCache } from '../lib/peerIdentityCache'
import { shortenPeerID } from '../lib/peer'
import { displayName } from '../lib/displayName'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(true)
  return (
    <div className="border-b border-border-0">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="w-full flex items-center gap-1 h-7 px-2 text-2xs font-medium uppercase tracking-[0.06em] text-text-1 hover:text-text-0 cursor-pointer"
      >
        {open ? (
          <ChevronDown size={10} strokeWidth={1.5} />
        ) : (
          <ChevronRight size={10} strokeWidth={1.5} />
        )}
        {title}
      </button>
      {open && <div className="pb-2">{children}</div>}
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[40%_60%] gap-2 px-2 py-0.5 text-sm items-baseline">
      <span className="text-text-1 text-right truncate">{label}</span>
      <span className={`text-text-0 break-all ${mono ? 'font-mono text-xs tnum' : ''}`}>{value}</span>
    </div>
  )
}

export function Inspector() {
  const location = useLocation()
  const navigate = useNavigate()
  const { data } = useWorkers(true)
  const cache = usePeerIdentityCache((s) => s.byPeerId)

  const target = location.pathname.startsWith('/peer/')
    ? decodeURIComponent(location.pathname.split('/')[2] ?? '')
    : null

  const raw = target ? data?.agents.find((a) => a.peer_id === target) : undefined
  const worker = raw ? mergeWithCache(raw, cache) : undefined

  return (
    <aside className="pane shrink-0 w-[260px] h-full overflow-y-auto bg-inspector border-l border-border-0" aria-label="Inspector">
      {!worker ? (
        <div className="h-full flex flex-col items-center justify-center gap-1 px-4 text-center">
          <div className="text-lg font-semibold text-text-1">No Selection</div>
          <div className="text-sm text-text-3">
            Select a worker on the Mesh Radar or pin an agent in Chat.
          </div>
        </div>
      ) : (
        <>
          <div className="px-2 py-2 border-b border-border-0">
            <div className="text-lg font-semibold text-text-0 truncate">{displayName(worker, cache[worker.peer_id])}</div>
            <div className="font-mono text-2xs text-text-1 truncate tnum">{shortenPeerID(worker.peer_id, 14, 6)}</div>
          </div>
          <Section title="Identity">
            <Row label="Author" value={worker.author?.trim() || 'Anonymous'} />
            <Row label="Model" value={worker.model?.trim() || 'n/a'} />
            <Row label="Capability" value={worker.agent_capability?.trim() || 'n/a'} mono />
            <Row label="Online" value={worker.online ? 'yes' : 'no'} />
          </Section>
          <Section title="Hardware">
            <Row label="CPU" value={`${Math.round(worker.cpu_usage_pct ?? 0)} %`} mono />
            <Row label="GPU" value={worker.has_gpu ? 'available' : 'none'} />
            <Row label="RAM free" value={`${worker.ram_free_gb.toFixed(1)} GB`} mono />
          </Section>
          <Section title="Queue">
            <Row label="Tasks" value={`${worker.current_tasks} / ${worker.max_tasks}`} mono />
            <Row label="Dispatch" value={worker.dispatch_allowed ? 'allowed' : 'refused'} />
            <Row label="Honesty" value={`${worker.honesty_score >= 0 ? '+' : ''}${worker.honesty_score.toFixed(2)}`} mono />
          </Section>
          <Section title="History">
            <div className="px-2 pt-1">
              <button
                onClick={() => navigate(`/peer/${encodeURIComponent(worker.peer_id)}`)}
                className="h-[22px] px-2.5 rounded-ctl bg-control text-sm text-text-0 hover:bg-control-hover transition-colors duration-150 cursor-pointer"
              >
                Open ledger history
              </button>
            </div>
          </Section>
        </>
      )}
    </aside>
  )
}

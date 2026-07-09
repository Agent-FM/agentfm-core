import { useEffect, useState } from 'react'
import { ChevronDown, ChevronRight, Trash2, Search } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'

type Scope = 'all' | 'agent' | 'errors'

function Group({ title, kv }: { title: string; kv: [string, string][] }) {
  const [open, setOpen] = useState(true)
  return (
    <div>
      <button
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center gap-1 h-6 px-1.5 text-sm text-text-0 hover:bg-white/[0.04] cursor-pointer"
        aria-expanded={open}
      >
        {open ? (
          <ChevronDown size={10} strokeWidth={1.5} className="text-text-1" />
        ) : (
          <ChevronRight size={10} strokeWidth={1.5} className="text-text-1" />
        )}
        {title}
      </button>
      {open &&
        kv.map(([k, v]) => (
          <div key={k} className="flex h-6 items-center pl-6 pr-2 gap-2 text-sm">
            <span className="text-text-1 shrink-0">{k}</span>
            <span className="font-mono text-xs text-text-0 truncate tnum ml-auto">{v}</span>
          </div>
        ))}
    </div>
  )
}

export function DebugArea() {
  const active = useUIStore((s) => s.activeProject())
  const backend = useBackend()
  const { data: about } = useAbout()
  const [lines, setLines] = useState<string[]>([])
  const [filter, setFilter] = useState('')
  const [scope, setScope] = useState<Scope>('all')

  useEffect(() => {
    let alive = true
    window.api?.backend
      .logs(300)
      .then((l) => { if (alive) setLines(l as string[]) })
      .catch(() => {})
    return () => { alive = false }
  }, [])

  const visible = lines.filter((l) => {
    if (scope === 'errors' && !/error|warn|fail/i.test(l)) return false
    if (scope === 'agent' && !/AGENTFM|task/i.test(l)) return false
    if (filter && !l.toLowerCase().includes(filter.toLowerCase())) return false
    return true
  })

  return (
    <div className="pane shrink-0 h-[240px] flex flex-col border-t border-border-0 bg-editor" aria-label="Debug area">
      <div className="flex-1 flex min-h-0">
        {/* variables view */}
        <div className="w-[40%] min-w-[180px] border-r border-border-0 overflow-y-auto py-0.5">
          <Group
            title="Project"
            kv={[
              ['name', active?.name ?? 'n/a'],
              ['mode', active ? (active.connectionMode === 'private' ? 'private swarm' : 'public mesh') : 'n/a'],
              ['floor', active ? active.reputationFloor.toFixed(2) : 'n/a'],
            ]}
          />
          <Group
            title="Backend"
            kv={[
              ['status', backend.ok ? 'healthy' : 'down'],
              ['workers', String(backend.online_workers ?? 0)],
              ['version', about?.version ?? 'n/a'],
            ]}
          />
          <Group
            title="Relay"
            kv={[
              ['connected', about?.relay_peer_id ? 'yes' : 'no'],
              ['ledger', String(about?.ledger_tree_size ?? 0)],
            ]}
          />
        </div>
        {/* console */}
        <pre className="flex-1 overflow-auto p-2 mono-console whitespace-pre-wrap text-text-1 m-0">
          {visible.length === 0 ? (
            <span className="text-text-2">Console is empty.</span>
          ) : (
            visible.join('')
          )}
        </pre>
      </div>
      {/* mini bar */}
      <div className="h-[26px] shrink-0 flex items-center gap-1.5 px-1.5 border-t border-border-0 bg-chrome">
        <div className="inline-flex rounded-ctl bg-raised p-px" role="group" aria-label="Console scope">
          {(['all', 'agent', 'errors'] as Scope[]).map((s) => (
            <button
              key={s}
              onClick={() => setScope(s)}
              aria-pressed={scope === s}
              className={`px-2 h-[18px] text-2xs rounded-[4px] capitalize transition-colors duration-150 cursor-pointer ${
                scope === s ? 'bg-control-selected text-text-0' : 'text-text-1 hover:text-text-0'
              }`}
            >
              {s === 'agent' ? 'Agent stream' : s === 'all' ? 'All Output' : 'Errors'}
            </button>
          ))}
        </div>
        <div className="flex-1" />
        <div className="w-[180px] h-[18px] flex items-center gap-1 rounded-ctl bg-bg-well border border-border-0 px-1.5">
          <Search size={9} strokeWidth={1.5} className="shrink-0 text-text-2" />
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter"
            aria-label="Filter console output"
            className="w-full bg-transparent text-2xs text-text-0 placeholder-text-2 outline-none"
          />
        </div>
        <button
          onClick={() => setLines([])}
          aria-label="Clear console"
          title="Clear console"
          className="p-1 text-text-1 hover:text-text-0 cursor-pointer"
        >
          <Trash2 size={12} strokeWidth={1.5} />
        </button>
      </div>
    </div>
  )
}

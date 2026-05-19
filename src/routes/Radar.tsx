import { useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import { Search, ChevronDown } from 'lucide-react'
import { useWorkers } from '../lib/query'
import { useUIStore } from '../lib/store'
import { AgentCard } from '../components/AgentCard'
import { Input } from '../components/primitives/Input'
import { Badge } from '../components/primitives/Badge'
import { EmptyRadar } from '../components/EmptyRadar'
import { RadarSkeleton } from '../components/RadarSkeleton'

type FilterPill = 'all' | 'trusted' | 'available' | 'capability'

export default function Radar() {
  const { data, isPending, error, refetch } = useWorkers(true)
  const navigate = useNavigate()
  const openDispatch = useUIStore((s) => s.openDispatch)
  const search = useUIStore((s) => s.searchTerm)
  const setSearch = useUIStore((s) => s.setSearchTerm)
  const [activeFilter, setActiveFilter] = useState<FilterPill>('all')
  const [capabilityFilter, setCapabilityFilter] = useState<string | null>(null)
  const [offlineOpen, setOfflineOpen] = useState(false)

  const agents = data?.agents ?? []
  const allCapabilities = useMemo(() => {
    const set = new Set<string>()
    agents.forEach((a) => a.agent_capability && set.add(a.agent_capability))
    return Array.from(set).sort()
  }, [agents])

  const filtered = useMemo(() => {
    return agents.filter((a) => {
      const matchesSearch =
        !search ||
        [a.name, a.peer_id, a.agent_image_ref, a.agent_image_digest, a.agent_capability].some(
          (f) => f && f.toLowerCase().includes(search.toLowerCase()),
        )
      if (!matchesSearch) return false
      switch (activeFilter) {
        case 'all': return true
        case 'trusted': return a.honesty_score > 0.3 && !a.is_equivocator
        case 'available': return a.online && a.dispatch_allowed && a.current_tasks < a.max_tasks
        case 'capability': return !capabilityFilter || a.agent_capability === capabilityFilter
      }
    })
  }, [agents, search, activeFilter, capabilityFilter])

  const online = filtered.filter((a) => a.online)
  const offline = filtered.filter((a) => !a.online)

  if (isPending) return <RadarSkeleton />
  if (error) {
    return (
      <div className="p-7">
        <div className="text-bad mb-3">{(error as Error).message}</div>
        <button onClick={() => refetch()} className="text-xs bg-bg-2 border border-border-0 rounded-md px-3 py-1.5">
          Retry
        </button>
      </div>
    )
  }
  if (agents.length === 0 && !search && activeFilter === 'all') {
    return (
      <div className="p-7">
        <Header />
        <EmptyRadar />
      </div>
    )
  }

  return (
    <div className="p-7 max-w-5xl">
      <Header />
      <div className="flex justify-between items-center mb-5 gap-3 flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-2" />
          <Input
            placeholder="Search agents…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8 w-80"
          />
        </div>
        <div className="flex gap-1.5 flex-wrap">
          {(['all', 'trusted', 'available', 'capability'] as FilterPill[]).map((f) => (
            <button
              key={f}
              onClick={() => setActiveFilter(f)}
              className={`relative text-xs px-3 py-1 rounded-full transition-colors ${
                activeFilter === f
                  ? 'text-accent gradient-border-cyan bg-accent-bg'
                  : 'bg-bg-2 border border-border-0 text-text-2 hover:text-text-0'
              }`}
            >
              {f === 'all' ? 'All' : f === 'trusted' ? 'Trusted' : f === 'available' ? 'Available' : 'Capability'}
            </button>
          ))}
          {activeFilter === 'capability' && allCapabilities.length > 0 && (
            <select
              value={capabilityFilter ?? ''}
              onChange={(e) => setCapabilityFilter(e.target.value || null)}
              className="text-xs bg-bg-2 border border-border-0 rounded-md px-2.5 py-1 text-text-1"
            >
              <option value="">All capabilities</option>
              {allCapabilities.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          )}
        </div>
      </div>

      <Section title="Online" count={online.length}>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          <AnimatePresence initial={false} mode="popLayout">
            {online.map((w) => (
              <motion.div key={w.peer_id} layout
                initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }}
                transition={{ type: 'spring', stiffness: 280, damping: 30 }}>
                <AgentCard worker={w} onHistory={() => navigate(`/peer/${w.peer_id}`)} onDispatch={() => openDispatch(w.peer_id)} />
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
        {online.length === 0 && <div className="text-sm text-text-2 py-3">No online agents match your filter.</div>}
      </Section>

      {offline.length > 0 && (
        <div className="mt-7">
          <button onClick={() => setOfflineOpen((v) => !v)}
            className="inline-flex items-center gap-1.5 text-2xs uppercase tracking-wider text-text-2 hover:text-text-0">
            <ChevronDown size={12} className={`transition-transform ${offlineOpen ? '' : '-rotate-90'}`} />
            <span>Offline</span>
            <span className="text-text-3">({offline.length})</span>
          </button>
          <AnimatePresence>
            {offlineOpen && (
              <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.2 }} className="overflow-hidden">
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-3 mt-3">
                  {offline.map((w) => (
                    <AgentCard key={w.peer_id} worker={w} onHistory={() => navigate(`/peer/${w.peer_id}`)} onDispatch={() => openDispatch(w.peer_id)} />
                  ))}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  )
}

function Header() {
  return (
    <div>
      <div className="flex items-baseline gap-3 mb-1.5">
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Agent Radar</h1>
        <Badge tone="cyan"><span className="animate-pulse-cyan inline-block w-1 h-1 rounded-full bg-accent mr-1" />LIVE</Badge>
      </div>
      <p className="text-text-2 mb-5">Every worker the mesh has heard of. Online updates in real time.</p>
    </div>
  )
}

function Section({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-2xs uppercase tracking-wider text-text-2 mb-3 flex items-center gap-2">
        {title}
        <span className="text-text-3">({count})</span>
      </div>
      {children}
    </div>
  )
}

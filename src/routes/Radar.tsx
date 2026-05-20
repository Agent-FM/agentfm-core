import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { AgentCard } from '../components/AgentCard'
import { useWorkers } from '../lib/query'
import { useUIStore } from '../lib/store'
import { EmptyState } from '../components/EmptyState'

export default function Radar() {
  const { data, isPending, error } = useWorkers(true)
  const agents = data?.agents ?? []
  const searchTerm = useUIStore((s) => s.searchTerm)
  const filterTrustedOnly = useUIStore((s) => s.filterTrustedOnly)

  const filtered = agents.filter((a) => {
    if (filterTrustedOnly && (a.honesty_score < 0 || a.is_equivocator)) return false
    if (searchTerm.trim() === '') return true
    const q = searchTerm.trim().toLowerCase()
    return (
      (a.name ?? '').toLowerCase().includes(q) ||
      (a.peer_id ?? '').toLowerCase().includes(q) ||
      (a.description ?? '').toLowerCase().includes(q)
    )
  })

  const onlineCount = agents.filter((a) => a.online).length
  const offlineCount = agents.length - onlineCount

  if (isPending) {
    return (
      <div className="p-7 text-text-2 text-[15px]">Scanning the mesh…</div>
    )
  }
  if (error) {
    return (
      <div className="p-7 text-bad text-[15px]">
        Couldn't reach the boss: {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="p-7 max-w-6xl">
      <SectionLabel>RADAR</SectionLabel>
      <HeroTitle accent="mesh">Your</HeroTitle>
      <p className="text-[17px] text-text-1 mt-2 mb-7">
        <span className="text-accent font-mono font-semibold">{onlineCount}</span>{' '}
        agents online ·{' '}
        <span className="text-accent font-mono font-semibold">{offlineCount}</span>{' '}
        known offline · gossip every 2s
      </p>

      {agents.length === 0 ? (
        <EmptyState />
      ) : filtered.length === 0 ? (
        <div className="text-text-2 text-[15px]">
          No agents match the current filter. Clear the search to see all{' '}
          <span className="text-accent font-mono">{agents.length}</span> known agents.
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {filtered.map((w) => (
            <AgentCard key={w.peer_id} worker={w} />
          ))}
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import { motion } from 'framer-motion'
import { Eye, EyeOff } from 'lucide-react'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { AgentCard } from '../components/AgentCard'
import { useWorkers } from '../lib/query'
import { useUIStore } from '../lib/store'
import { EmptyRadar } from '../components/EmptyRadar'
import { staggerItem } from '../lib/motion'

export default function Radar() {
  const { data, isPending, error } = useWorkers(true)
  const agents = data?.agents ?? []
  const searchTerm = useUIStore((s) => s.searchTerm)
  const filterTrustedOnly = useUIStore((s) => s.filterTrustedOnly)
  const [hideOffline, setHideOffline] = useState(false)

  const filtered = agents.filter((a) => {
    if (hideOffline && !a.online) return false
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
      <div className="p-6 text-text-2 text-[15px]">Scanning the mesh…</div>
    )
  }
  if (error) {
    return (
      <div className="p-6 text-bad text-[15px]">
        Couldn't reach the boss: {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="p-6 max-w-6xl">
      <SectionLabel>RADAR</SectionLabel>
      <HeroTitle accent="mesh">Your</HeroTitle>
      <div className="flex items-center justify-between gap-4 mt-2 mb-6">
        <p className="text-[17px] text-text-1">
          <span className="text-accent font-mono font-semibold tabular-nums">{onlineCount}</span>{' '}
          agents online ·{' '}
          <span className="text-accent font-mono font-semibold tabular-nums">{offlineCount}</span>{' '}
          known offline
        </p>
        {offlineCount > 0 && (
          <button
            onClick={() => setHideOffline((v) => !v)}
            className="shrink-0 inline-flex items-center gap-1.5 rounded-full border border-border-1 bg-bg-1 px-3 py-1.5 text-xs text-text-1 hover:text-text-0 hover:border-accent/45 transition-colors"
          >
            {hideOffline ? <Eye size={13} /> : <EyeOff size={13} />}
            <span>{hideOffline ? 'Show offline' : 'Hide offline'}</span>
          </button>
        )}
      </div>

      {agents.length === 0 ? (
        <EmptyRadar />
      ) : filtered.length === 0 ? (
        <div className="text-text-2 text-[15px]">
          No agents match the current filter. Clear the search to see all{' '}
          <span className="text-accent font-mono tabular-nums">{agents.length}</span> known agents.
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {filtered.map((w, i) => (
            <motion.div key={w.peer_id} {...staggerItem(i)}>
              <AgentCard worker={w} />
            </motion.div>
          ))}
        </div>
      )}
    </div>
  )
}

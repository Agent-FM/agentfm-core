import { useState } from 'react'
import { motion } from 'framer-motion'
import { Eye, EyeOff } from 'lucide-react'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { AgentCard } from '../components/AgentCard'
import { useWorkers } from '../lib/query'
import { useUIStore } from '../lib/store'
import { usePeerIdentityCache } from '../lib/peerIdentityCache'
import { hasResolvableName } from '../lib/displayName'
import { EmptyRadar } from '../components/EmptyRadar'
import { easeOut } from '../lib/motion'

export default function Radar() {
  const { data, isPending, error } = useWorkers(true)
  const cache = usePeerIdentityCache((s) => s.byPeerId)
  const searchTerm = useUIStore((s) => s.searchTerm)
  const filterTrustedOnly = useUIStore((s) => s.filterTrustedOnly)
  const [hideOffline, setHideOffline] = useState(false)

  const agents = (data?.agents ?? []).filter(
    (a) => a.online || hasResolvableName(a, cache[a.peer_id]),
  )

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
      <div className="p-4 text-text-2 text-sm">Scanning the mesh…</div>
    )
  }
  if (error) {
    return (
      <div className="p-4 text-bad text-sm">
        Couldn't reach the boss: {(error as Error).message}
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between gap-4 px-4 pt-4 pb-3">
        <div className="min-w-0">
          <HeroTitle accent="mesh">Your</HeroTitle>
          <div className="flex items-center gap-4 text-2xs text-text-1 mt-0.5">
            <span>
              <span className="text-accent font-mono tabular-nums">{onlineCount}</span> online
            </span>
            <span>
              <span className="font-mono tabular-nums">{offlineCount}</span> offline
            </span>
          </div>
        </div>
        {offlineCount > 0 && (
          <button
            onClick={() => setHideOffline((v) => !v)}
            className="shrink-0 inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-ctl bg-control hover:bg-control-hover text-xs text-text-1 hover:text-text-0 transition-colors duration-150"
          >
            {hideOffline ? <Eye size={13} /> : <EyeOff size={13} />}
            <span>{hideOffline ? 'Show offline' : 'Hide offline'}</span>
          </button>
        )}
      </div>

      {agents.length === 0 ? (
        <EmptyRadar />
      ) : filtered.length === 0 ? (
        <div className="px-4 text-text-2 text-sm">
          No agents match the current filter. Clear the search to see all{' '}
          <span className="text-accent font-mono tabular-nums">{agents.length}</span> known agents.
        </div>
      ) : (
        <div
          className="px-4 pb-4 grid gap-3 items-stretch"
          style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))' }}
        >
          {filtered.map((w, i) => (
            <motion.div
              key={w.peer_id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, ease: easeOut, delay: Math.min(i, 10) * 0.03 }}
              className="h-full"
            >
              <AgentCard worker={w} />
            </motion.div>
          ))}
        </div>
      )}
    </div>
  )
}

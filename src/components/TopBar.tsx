import { Link } from 'react-router-dom'
import { useWorkers } from '../lib/query'
import { useBackend } from '../hooks/useBackend'

export function TopBar() {
  const { data } = useWorkers(true)
  const backend = useBackend()

  const onlineCount = data?.online_count ?? 0
  const offlineCount = data?.offline_count ?? 0
  const relayStatus = backend.ok ? '✓ stable' : '✗ offline'

  return (
    <header className="h-10 bg-bg-1 border-b border-border-0 px-4 flex items-center text-xs text-text-2">
      <div className="flex items-center gap-2 font-semibold text-text-0">
        <span className="w-2 h-2 rounded-full bg-accent shadow-[0_0_6px_var(--accent)] animate-pulse" />
        AgentFM
      </div>
      <div className="flex-1" />
      <Link
        to="/status"
        className="px-3 py-1 rounded-full bg-bg-2 border border-border-0 text-text-1 hover:border-border-1 transition-colors"
      >
        {onlineCount} online · {offlineCount} offline · relay {relayStatus}
      </Link>
    </header>
  )
}

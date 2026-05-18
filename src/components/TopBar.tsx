import { useNavigate } from 'react-router-dom'
import { useWorkers } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { ProjectPill } from './projects/ProjectPill'

export function TopBar() {
  const navigate = useNavigate()
  const { data } = useWorkers(false)
  const backend = useBackend()
  const online = data?.online_count ?? 0
  const offline = data?.offline_count ?? 0
  const relayOk = backend.ok

  return (
    <header className="h-10 border-b border-border-0 bg-bg-0 grid grid-cols-3 items-center px-3 select-none">
      <div className="justify-self-start">
        <ProjectPill />
      </div>
      <div className="justify-self-center text-sm font-semibold tracking-tight text-text-0">
        AgentFM
      </div>
      <div className="justify-self-end">
        <button
          onClick={() => navigate('/status')}
          className="text-2xs text-text-2 hover:text-text-0 transition-colors"
        >
          {online} online · {offline} offline · relay {relayOk ? '✓ stable' : '⚠ down'}
        </button>
      </div>
    </header>
  )
}

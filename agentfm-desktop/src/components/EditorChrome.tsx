import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { shortenPeerID } from '../lib/peer'

const LABELS: Record<string, string> = {
  radar: 'Mesh Radar',
  dashboard: 'Dashboard',
  chat: 'Chat',
  activity: 'Activity',
  assets: 'Artifacts',
  status: 'Status',
  'getting-started': 'Getting Started',
  settings: 'Settings',
  developer: 'Developer',
  peer: 'Mesh Radar',
}

export function EditorChrome() {
  const location = useLocation()
  const navigate = useNavigate()
  const params = useParams()
  const active = useUIStore((s) => s.activeProject())

  const seg = location.pathname.split('/').filter(Boolean)
  const section = seg[0] ?? 'radar'
  const sectionLabel = LABELS[section] ?? section
  const sectionPath = section === 'peer' ? '/radar' : `/${section}`

  let leaf: string | null = null
  if (section === 'peer' && seg[1]) leaf = shortenPeerID(seg[1], 10, 4)
  else if (section === 'chat' && params.sessionId) leaf = 'Session'

  return (
    <div className="shrink-0 select-none">
      {/* tab bar */}
      <div className="h-[30px] flex items-stretch bg-chrome border-b border-border-0">
        <div className="flex items-center gap-1.5 px-3 bg-editor border-r border-border-0 text-sm text-text-0 max-w-[240px]">
          <span className="truncate">{sectionLabel}</span>
        </div>
        <div className="flex-1" />
      </div>
      {/* jump bar */}
      <div className="h-[26px] flex items-center gap-0.5 px-2.5 bg-editor border-b border-border-0 text-2xs text-text-1 overflow-hidden whitespace-nowrap">
        <span className="truncate max-w-[160px]">{active?.name ?? 'AgentFM'}</span>
        <ChevronRight size={10} strokeWidth={1.5} className="shrink-0 text-text-2" />
        <button
          onClick={() => navigate(sectionPath)}
          className="hover:text-text-0 transition-colors cursor-pointer truncate"
        >
          {sectionLabel}
        </button>
        {leaf && (
          <>
            <ChevronRight size={10} strokeWidth={1.5} className="shrink-0 text-text-2" />
            <span className="font-mono text-text-0 truncate">{leaf}</span>
          </>
        )}
      </div>
    </div>
  )
}

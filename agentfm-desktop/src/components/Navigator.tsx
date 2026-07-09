import { useRef } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import {
  Radio, Gauge, MessageSquare, Activity as ActivityIcon, FolderDown,
  HeartPulse, BookOpen, Settings as SettingsIcon, Terminal,
} from 'lucide-react'
import { useUIStore } from '../lib/store'
import markUrl from '../assets/logo-mark.png'

// Ordered by the user journey: onboarding first, then the core dispatch
// workflow, then monitoring, then system.
const WORKSPACE = [
  { to: '/getting-started', label: 'Getting Started', icon: BookOpen },
  { to: '/radar', label: 'Mesh Radar', icon: Radio },
  { to: '/chat', label: 'Chat', icon: MessageSquare },
  { to: '/assets', label: 'Artifacts', icon: FolderDown },
]
const MONITOR = [
  { to: '/dashboard', label: 'Dashboard', icon: Gauge },
  { to: '/activity', label: 'Activity', icon: ActivityIcon },
  { to: '/status', label: 'Status', icon: HeartPulse },
]
const SYSTEM = [
  { to: '/settings', label: 'Settings', icon: SettingsIcon },
  { to: '/developer', label: 'Developer', icon: Terminal },
]
function isRouteActive(pathname: string, to: string): boolean {
  return (
    pathname === to ||
    (to === '/chat' && pathname.startsWith('/chat')) ||
    (to === '/radar' && pathname.startsWith('/peer/'))
  )
}

interface Props {
  width: number
  onResize: (w: number) => void
  onCollapse: () => void
}

export function Navigator({ width, onResize, onCollapse }: Props) {
  const active = useUIStore((s) => s.activeProject())
  const location = useLocation()
  const dragging = useRef(false)

  function startDrag(e: React.PointerEvent) {
    dragging.current = true
    const startX = e.clientX
    const startW = width
    const move = (ev: PointerEvent) => {
      if (!dragging.current) return
      onResize(Math.min(400, Math.max(200, startW + (ev.clientX - startX))))
    }
    const up = () => {
      dragging.current = false
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  function renderRows(items: typeof WORKSPACE) {
    return items.map(({ to, label, icon: Icon }) => {
      const sel = isRouteActive(location.pathname, to)
      return (
        <NavLink
          key={to}
          to={to}
          aria-current={sel ? 'page' : undefined}
          className={`flex items-center gap-2 h-7 px-2 rounded-[5px] text-sm transition-colors duration-150 ${
            sel ? 'row-selected text-text-0 font-medium' : 'text-text-1 hover:bg-white/[0.04] hover:text-text-0'
          }`}
        >
          <Icon size={15} strokeWidth={1.5} className={`shrink-0 ${sel ? 'text-accent' : 'text-text-1'}`} />
          <span className="truncate">{label}</span>
        </NavLink>
      )
    })
  }

  return (
    <aside
      className="pane relative shrink-0 h-full flex flex-col bg-navigator border-r border-border-0"
      style={{ width }}
      aria-label="Navigator"
    >
      {/* header */}
      <div className="h-9 shrink-0 flex items-center gap-2 px-3 border-b border-border-0">
        <img src={markUrl} alt="" aria-hidden="true" className="h-4 w-auto select-none" draggable={false} />
        <span className="text-sm font-semibold text-text-0 select-none">
          Agent<span className="text-accent">FM</span>
        </span>
      </div>

      {/* list */}
      {active && (
        <nav aria-label="Primary" className="flex-1 overflow-y-auto py-1.5 px-1.5">
          {renderRows(WORKSPACE)}
          <div className="mt-3 pt-2.5 border-t border-border-0">
            <div className="mb-1 px-2 flex items-center gap-1.5 text-2xs font-semibold uppercase tracking-[0.06em] text-accent select-none">
              <span className="w-1.5 h-1.5 rounded-full bg-accent" />
              Monitoring
            </div>
            {renderRows(MONITOR)}
          </div>
          <div className="mt-3 pt-2.5 border-t border-border-0">
            <div className="mb-1 px-2 flex items-center gap-1.5 text-2xs font-semibold uppercase tracking-[0.06em] text-ok select-none">
              <span className="w-1.5 h-1.5 rounded-full bg-ok" />
              System
            </div>
            {renderRows(SYSTEM)}
          </div>
        </nav>
      )}

      {/* splitter */}
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize navigator"
        onPointerDown={startDrag}
        onDoubleClick={onCollapse}
        className="absolute top-0 -right-[2px] w-[5px] h-full cursor-col-resize z-10"
      />
    </aside>
  )
}

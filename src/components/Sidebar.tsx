import { NavLink } from 'react-router-dom'
import { ProjectList } from './projects/ProjectList'

const items = [
  { to: '/radar', icon: '🛰', label: 'Radar' },
  { to: '/chat', icon: '💬', label: 'Chat' },
  { to: '/activity', icon: '📜', label: 'Activity' },
  { to: '/status', icon: '📊', label: 'Status' },
]

function navClass({ isActive }: { isActive: boolean }) {
  return `flex items-center gap-2.5 px-3 py-2 rounded-md text-sm text-text-1 hover:bg-bg-2 hover:text-text-0 transition-colors ${
    isActive ? '!bg-accent-bg !text-accent' : ''
  }`
}

export function Sidebar() {
  return (
    <nav className="w-56 shrink-0 bg-bg-1 border-r border-border-0 flex flex-col">
      <ProjectList />

      <div className="px-2 py-3 flex flex-col gap-0.5">
        {items.map((it) => (
          <NavLink key={it.to} to={it.to} className={navClass}>
            <span className="text-base leading-none w-5 text-center">{it.icon}</span>
            <span>{it.label}</span>
          </NavLink>
        ))}
      </div>

      <div className="mt-auto px-2 pb-3">
        <NavLink to="/settings" className={navClass}>
          <span className="text-base leading-none w-5 text-center">⚙</span>
          <span>Settings</span>
        </NavLink>
      </div>
    </nav>
  )
}

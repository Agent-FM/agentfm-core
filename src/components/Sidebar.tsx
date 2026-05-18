import { NavLink } from 'react-router-dom'
import { ProjectList } from './projects/ProjectList'

const items = [
  { to: '/radar', icon: '🛰', label: 'Radar' },
  { to: '/chat', icon: '💬', label: 'Chat' },
  { to: '/activity', icon: '📜', label: 'Activity' },
  { to: '/status', icon: '📊', label: 'Status' },
]

function navClass({ isActive }: { isActive: boolean }) {
  return `w-10 h-10 rounded-lg flex items-center justify-center text-text-2 hover:bg-bg-2 transition-colors text-base ${
    isActive ? '!bg-accent-bg !text-accent' : ''
  }`
}

export function Sidebar() {
  return (
    <nav className="w-14 bg-bg-1 border-r border-border-0 flex flex-col items-center py-3 gap-1">
      <ProjectList />
      <div className="flex flex-col gap-1">
        {items.map((it) => (
          <NavLink key={it.to} to={it.to} className={navClass} title={it.label}>
            {it.icon}
          </NavLink>
        ))}
      </div>
      <div className="mt-auto">
        <NavLink to="/settings" className={navClass} title="Settings">
          ⚙
        </NavLink>
      </div>
    </nav>
  )
}

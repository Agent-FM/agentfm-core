import { NavLink } from 'react-router-dom'
import { useUIStore } from '../lib/store'

const tabs = [
  { to: '/radar', label: 'Radar' },
  { to: '/chat', label: 'Chat' },
  { to: '/activity', label: 'Activity' },
  { to: '/status', label: 'Status' },
]

function tabClass({ isActive }: { isActive: boolean }) {
  return `px-4 py-2 text-sm border-b-2 transition-colors ${
    isActive
      ? 'border-accent text-text-0'
      : 'border-transparent text-text-2 hover:text-text-0'
  }`
}

export function TabStrip() {
  const active = useUIStore((s) => s.activeProject())
  if (!active) return null

  return (
    <div className="border-b border-border-0 bg-bg-0 px-3 flex gap-1">
      {tabs.map((t) => (
        <NavLink key={t.to} to={t.to} className={tabClass}>
          {t.label}
        </NavLink>
      ))}
    </div>
  )
}

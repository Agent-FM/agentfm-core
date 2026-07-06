import { NavLink, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'

const tabs = [
  { to: '/radar', label: 'Radar' },
  { to: '/getting-started', label: 'Getting Started' },
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/chat', label: 'Chat' },
  { to: '/activity', label: 'Activity' },
  { to: '/assets', label: 'Assets' },
  { to: '/status', label: 'Status' },
  { to: '/settings', label: 'Settings' },
  { to: '/developer', label: 'Developer' },
]

export function TabStrip() {
  const active = useUIStore((s) => s.activeProject())
  const location = useLocation()
  if (!active) return null

  return (
    <div className="border-b border-border-0 bg-bg-0 px-3 flex gap-1 relative overflow-x-auto no-scrollbar">
      {tabs.map((t) => {
        const isActive = location.pathname === t.to || (t.to === '/chat' && location.pathname.startsWith('/chat'))
        return (
          <NavLink
            key={t.to}
            to={t.to}
            className={`relative shrink-0 whitespace-nowrap rounded-lg px-4 py-2.5 text-[14px] transition-colors ${
              isActive ? 'text-accent font-semibold' : 'text-text-2 hover:text-text-0 hover:bg-white/[0.03]'
            }`}
          >
            {t.label}
            {isActive && (
              <motion.div
                layoutId="tab-indicator"
                transition={{ type: 'spring', stiffness: 380, damping: 32 }}
                className="absolute bottom-0 left-2 right-2 h-[2px] bg-accent"
              />
            )}
          </NavLink>
        )
      })}
    </div>
  )
}

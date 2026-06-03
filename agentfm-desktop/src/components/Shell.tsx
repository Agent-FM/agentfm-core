import { Outlet, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { TopBar } from './TopBar'
import { TabStrip } from './TabStrip'
import { EmptyState } from './EmptyState'
import { SettingsFooter } from './SettingsFooter'
import { RouteErrorBoundary } from './RouteErrorBoundary'
import { RoutePage } from './primitives/RoutePage'
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts'

export function Shell() {
  useGlobalShortcuts()
  const loc = useLocation()
  const active = useUIStore((s) => s.activeProject())

  return (
    <div className="h-screen flex flex-col bg-bg-0 text-text-0 font-sans">
      <TopBar />
      <TabStrip />
      <main className="flex-1 overflow-hidden flex flex-col">
        {!active ? (
          <EmptyState />
        ) : (
          <motion.div
            key={loc.pathname}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.18 }}
            className="h-full overflow-auto"
          >
            <RouteErrorBoundary>
              <RoutePage>
                <Outlet />
              </RoutePage>
            </RouteErrorBoundary>
          </motion.div>
        )}
      </main>
      <SettingsFooter />
    </div>
  )
}

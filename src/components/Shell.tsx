import { Outlet, useLocation } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { RouteErrorBoundary } from './RouteErrorBoundary'
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts'

export function Shell() {
  useGlobalShortcuts()
  const loc = useLocation()

  return (
    <div className="h-screen flex flex-col bg-bg-0 text-text-0 font-sans">
      <TopBar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <AnimatePresence mode="wait">
            <motion.div
              key={loc.pathname}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.18 }}
              className="h-full"
            >
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </div>
  )
}

import { useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { Toolbar } from './Toolbar'
import { Navigator } from './Navigator'
import { EditorChrome } from './EditorChrome'
import { DebugArea } from './DebugArea'
import { Inspector } from './Inspector'
import { EmptyState } from './EmptyState'
import { SettingsFooter } from './SettingsFooter'
import { RouteErrorBoundary } from './RouteErrorBoundary'
import { RoutePage } from './primitives/RoutePage'
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts'

export function Shell() {
  useGlobalShortcuts()
  const loc = useLocation()
  const active = useUIStore((s) => s.activeProject())

  const [showNavigator, setShowNavigator] = useState(true)
  const [showDebug, setShowDebug] = useState(false)
  const [showInspector, setShowInspector] = useState(false)
  const [navWidth, setNavWidth] = useState(260)

  return (
    <div className="h-screen flex flex-col text-text-0 font-sans text-sm bg-editor">
      <Toolbar
        showNavigator={showNavigator}
        showDebug={showDebug}
        showInspector={showInspector}
        onToggleNavigator={() => setShowNavigator((v) => !v)}
        onToggleDebug={() => setShowDebug((v) => !v)}
        onToggleInspector={() => setShowInspector((v) => !v)}
      />
      <div className="flex-1 flex min-h-0">
        <div
          className="shrink-0 overflow-hidden transition-[width] duration-200 ease-out"
          style={{ width: showNavigator ? navWidth : 0 }}
        >
          <Navigator
            width={navWidth}
            onResize={setNavWidth}
            onCollapse={() => setShowNavigator(false)}
          />
        </div>
        <div className="flex-1 min-w-0 flex flex-col bg-editor">
          <EditorChrome />
          <main className="flex-1 overflow-hidden flex flex-col">
            {!active ? (
              <EmptyState />
            ) : (
              <motion.div
                key={loc.pathname}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.15, ease: 'easeOut' }}
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
          <div
            className="shrink-0 overflow-hidden transition-[height] duration-200 ease-out"
            style={{ height: showDebug ? 240 : 0 }}
          >
            <DebugArea />
          </div>
          <SettingsFooter />
        </div>
        <div
          className="shrink-0 overflow-hidden transition-[width] duration-200 ease-out"
          style={{ width: showInspector ? 260 : 0 }}
        >
          <Inspector />
        </div>
      </div>
    </div>
  )
}

import { AnimatePresence, motion } from 'framer-motion'
import { X, FileText } from 'lucide-react'
import { useState } from 'react'
import { useUIStore } from '../lib/store'
import { useBackend } from '../hooks/useBackend'
import { useAbout } from '../lib/query'
import { SegGroup } from './primitives/SegGroup'
import { Button } from './primitives/Button'
import { StatusDot } from './primitives/StatusDot'
import { LogsModal } from './status/LogsModal'

export function SettingsSheet() {
  const open = useUIStore((s) => s.isSettingsSheetOpen)
  const close = useUIStore((s) => s.closeSettingsSheet)
  const theme = useUIStore((s) => s.theme)
  const setTheme = useUIStore((s) => s.setTheme)
  const backend = useBackend()
  const { data: about } = useAbout()
  const [showLogs, setShowLogs] = useState(false)

  return (
    <>
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/75 backdrop-blur-sm z-[65] flex justify-end"
            onClick={close}
          >
            <motion.div
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              exit={{ x: '100%' }}
              transition={{ type: 'spring', stiffness: 320, damping: 30 }}
              onClick={(e) => e.stopPropagation()}
              className="w-[420px] h-full bg-bg-1 border-l border-border-0 p-7 overflow-auto"
            >
              <div className="flex justify-between items-start mb-7">
                <h2 className="text-xl font-semibold tracking-tight text-text-0">Settings</h2>
                <button onClick={close} className="text-text-2 hover:text-text-0">
                  <X size={18} />
                </button>
              </div>

              <div className="mb-7 bg-bg-2 border border-border-0 rounded-xl p-4">
                <div className="text-2xs uppercase tracking-wider text-text-2 mb-2">Backend</div>
                <div className="flex items-center gap-2 text-sm">
                  <StatusDot tone={backend.ok ? 'cyan' : 'rose'} pulse={backend.ok} />
                  <span className="text-text-0">{backend.ok ? 'Healthy' : 'Down'}</span>
                  <span className="ml-auto text-2xs text-text-2 font-mono">v{about?.version ?? '…'}</span>
                </div>
                <div className="mt-2 text-2xs text-text-2 font-mono">
                  {backend.online_workers} online worker{backend.online_workers === 1 ? '' : 's'}
                </div>
                <div className="mt-4">
                  <Button onClick={() => setShowLogs(true)}>
                    <FileText size={12} />
                    <span>View logs</span>
                  </Button>
                </div>
              </div>

              <label className="block text-2xs uppercase tracking-wider text-text-2 mb-2">Theme</label>
              <SegGroup
                options={[
                  { value: 'dark', label: 'Dark' },
                  { value: 'light', label: 'Light' },
                  { value: 'auto', label: 'Auto' },
                ]}
                value={theme}
                onChange={setTheme}
              />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

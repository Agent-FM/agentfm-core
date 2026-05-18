import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { SegGroup } from './primitives/SegGroup'

export function SettingsSheet() {
  const open = useUIStore((s) => s.isSettingsSheetOpen)
  const close = useUIStore((s) => s.closeSettingsSheet)
  const theme = useUIStore((s) => s.theme)
  const setTheme = useUIStore((s) => s.setTheme)

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/65 backdrop-blur-sm z-[65] flex justify-end"
          onClick={close}
        >
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[380px] h-full bg-bg-1 border-l border-border-0 p-6"
          >
            <div className="flex justify-between items-start mb-6">
              <h2 className="text-xl font-semibold text-text-0">Settings</h2>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-2">Theme</label>
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
  )
}

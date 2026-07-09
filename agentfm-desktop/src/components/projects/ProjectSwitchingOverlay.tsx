import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'

export function ProjectSwitchingOverlay() {
  const show = useUIStore((s) => s.isProjectSwitching)
  const active = useUIStore((s) => s.activeProject())

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          transition={{ duration: 0.18 }}
          className="fixed inset-0 z-[80] flex items-center justify-center bg-bg-0/95"
        >
          <div className="text-center">
            <div className="relative w-8 h-8 mx-auto">
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 1.2, repeat: Infinity, ease: 'linear' }}
                className="absolute inset-0 rounded-full border-2 border-accent/20 border-t-accent"
              />
            </div>
            <div className="mt-4 text-text-1 text-sm">
              Switching to <span className="text-accent font-semibold">{active?.name ?? '…'}</span>…
            </div>
            <div className="mt-1 text-2xs text-text-2 font-mono">restarting backend with the new relay</div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

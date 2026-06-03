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
          className="fixed inset-0 z-[80] flex items-center justify-center bg-bg-0/90 backdrop-blur-xl"
        >
          <div className="text-center">
            <div className="relative w-16 h-16 mx-auto">
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 1.2, repeat: Infinity, ease: 'linear' }}
                className="absolute inset-0"
                style={{
                  background: 'conic-gradient(from 0deg, transparent 0%, #22d3ee 30%, #a855f7 70%, transparent 100%)',
                  borderRadius: '50%',
                  mask: 'radial-gradient(transparent 60%, black 62%)',
                  WebkitMask: 'radial-gradient(transparent 60%, black 62%)',
                }}
              />
            </div>
            <div className="mt-5 text-text-1 text-base">
              Switching to <span className="text-accent glow-text-cyan font-semibold">{active?.name ?? '…'}</span>…
            </div>
            <div className="mt-1 text-2xs text-text-2 font-mono">restarting backend with the new relay</div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

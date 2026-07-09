import { motion } from 'framer-motion'
import { Zap, Satellite } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { Button } from './primitives/Button'

export function EmptyState() {
  const openWizard = useUIStore((s) => s.openCreateWizard)

  return (
    <div className="relative flex-1 flex items-center justify-center p-8 overflow-hidden bg-editor">
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.15, ease: 'easeOut' }}
        className="relative max-w-md text-center"
      >
        <div className="mb-4 inline-flex items-center justify-center w-10 h-10 rounded-ctl bg-bg-well border border-border-0 text-text-2">
          <Satellite size={20} strokeWidth={1.5} />
        </div>
        <h1 className="text-lg font-semibold text-text-1">Welcome to AgentFM</h1>
        <p className="text-sm text-text-3 mt-2 mb-5 leading-relaxed">
          Create your first project to get started. A project pairs a name with a relay; you can
          add more later.
        </p>
        <Button onClick={openWizard}>
          <Zap size={14} strokeWidth={1.5} />
          <span>Create project</span>
        </Button>
      </motion.div>
    </div>
  )
}

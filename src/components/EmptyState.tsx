import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { Button } from './primitives/Button'

export function EmptyState() {
  const openWizard = useUIStore((s) => s.openCreateWizard)

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="flex-1 flex items-center justify-center p-8"
    >
      <div className="max-w-md text-center bg-bg-1 border border-border-0 rounded-2xl p-10">
        <div className="text-5xl mb-4">📁</div>
        <h1 className="text-xl font-semibold text-text-0">Welcome to AgentFM</h1>
        <p className="text-text-2 mt-2 mb-6">
          Create your first project to get started. A project pairs a name with a relay; you can
          add more later.
        </p>
        <Button variant="primary" onClick={openWizard}>
          Create project
        </Button>
      </div>
    </motion.div>
  )
}

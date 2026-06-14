import { motion } from 'framer-motion'
import { Zap } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { Button } from './primitives/Button'

export function EmptyState() {
  const openWizard = useUIStore((s) => s.openCreateWizard)

  return (
    <div className="relative flex-1 flex items-center justify-center p-8 overflow-hidden">
      <div
        className="absolute inset-0 pointer-events-none animate-drift"
        style={{
          background:
            'radial-gradient(circle at 20% 0%, rgba(34,211,238,.18), transparent 45%), radial-gradient(circle at 80% 100%, rgba(34,211,238,.14), transparent 45%)',
        }}
      />
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 240, damping: 28 }}
        className="relative max-w-md text-center bg-bg-1 border border-border-0 rounded-2xl p-12"
      >
        <div className="text-6xl mb-5 animate-float inline-block">🛰</div>
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Welcome to <span className="text-accent">AgentFM</span></h1>
        <p className="text-text-2 mt-3 mb-7 leading-relaxed">
          Create your first project to get started. A project pairs a name with a relay; you can
          add more later.
        </p>
        <Button variant="primary" onClick={openWizard}>
          <Zap size={14} />
          <span>Create project</span>
        </Button>
      </motion.div>
    </div>
  )
}

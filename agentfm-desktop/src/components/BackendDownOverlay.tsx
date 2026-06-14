import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { toast } from 'sonner'
import { AlertOctagon } from 'lucide-react'
import { Button } from './primitives/Button'

export function BackendDownOverlay({ show }: { show: boolean }) {
  const [logs, setLogs] = useState<string[]>([])
  const [showLogs, setShowLogs] = useState(false)

  async function loadLogs() {
    setShowLogs(true)
    const lines = await window.api.backend.logs(200)
    setLogs(lines as string[])
  }

  async function restart() {
    try {
      await window.api.backend.restart()
      toast.success('Backend restarted successfully')
    } catch (err) {
      toast.error('Failed to restart backend: ' + (err as Error).message)
      console.error(err)
    }
  }

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-[90] flex items-center justify-center bg-gradient-to-b from-bad/15 to-bg-0 backdrop-blur-md p-8"
        >
          <div className="max-w-xl w-full text-center">
            <AlertOctagon size={80} className="text-bad mx-auto animate-pulse" />
            <h2 className="text-2xl font-semibold tracking-tight text-text-0 mt-4">Backend stopped</h2>
            <p className="text-text-2 mb-6 mt-2">
              The bundled agentfm backend isn't responding. You can view logs to diagnose, or
              attempt a restart.
            </p>
            <div className="flex gap-3 mb-6 justify-center">
              <Button variant="primary" onClick={restart}>
                Restart backend
              </Button>
              <Button onClick={loadLogs}>View logs</Button>
            </div>
            {showLogs && (
              <pre className="text-[10px] font-mono text-text-2 bg-bg-1 border border-border-0 rounded p-3 max-h-80 overflow-auto text-left">
                {logs.join('')}
              </pre>
            )}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

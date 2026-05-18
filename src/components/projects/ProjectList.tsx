import { useUIStore } from '../../lib/store'
import { motion } from 'framer-motion'

const COLOR_HEX: Record<string, string> = {
  emerald: '#10b981',
  violet: '#8b5cf6',
  rose: '#f43f5e',
  cyan: '#22d3ee',
  amber: '#f59e0b',
}

export function ProjectList() {
  const projects = useUIStore((s) => s.projects)
  const activeId = useUIStore((s) => s.activeProjectId)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)
  const openWizard = useUIStore((s) => s.openCreateWizard)

  async function switchTo(id: string) {
    if (id === activeId) return
    setSwitching(true)
    const project = projects.find((p) => p.id === id)
    if (!project) {
      setSwitching(false)
      return
    }
    await window.api.settings.set('activeProjectId', id)
    useUIStore.setState({ activeProjectId: id })
    try {
      await window.api.backend.restart({
        apiPort: useUIStore.getState().apiPort,
        reputationFloor: project.reputationFloor,
        relayMultiaddr: project.relayMultiaddr ?? undefined,
      })
    } catch (e) {
      console.warn('project switch: backend restart failed', e)
    } finally {
      setSwitching(false)
    }
  }

  return (
    <div className="px-2 py-3 border-b border-border-0">
      <div className="text-2xs uppercase tracking-wider text-text-2 px-2 mb-2">Projects</div>
      <div className="space-y-1">
        {projects.map((p) => {
          const active = p.id === activeId
          return (
            <motion.button
              key={p.id}
              whileTap={{ scale: 0.97 }}
              onClick={() => switchTo(p.id)}
              className={`w-full text-left flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors ${
                active ? 'bg-bg-2 text-text-0' : 'text-text-1 hover:bg-bg-2/60'
              }`}
            >
              <span className="text-base leading-none">{p.icon}</span>
              <span className="flex-1 truncate">{p.name}</span>
              <span
                className="w-1.5 h-1.5 rounded-full"
                style={{ background: COLOR_HEX[p.color] ?? '#10b981' }}
              />
            </motion.button>
          )
        })}
      </div>
      <button
        onClick={openWizard}
        className="w-full text-left mt-2 px-2 py-1.5 rounded-md text-xs text-text-2 hover:text-accent hover:bg-bg-2/60"
      >
        + new project
      </button>
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { toast } from 'sonner'
import { ChevronDown, Plus, Trash2 } from 'lucide-react'
import { useUIStore } from '../../lib/store'

export function ProjectDropdown() {
  const projects = useUIStore((s) => s.projects)
  const activeId = useUIStore((s) => s.activeProjectId)
  const active = useUIStore((s) => s.activeProject())
  const switchProject = useUIStore((s) => s.switchProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const openWizard = useUIStore((s) => s.openCreateWizard)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    if (open) {
      document.addEventListener('mousedown', onDown)
      document.addEventListener('keydown', onKey)
      return () => {
        document.removeEventListener('mousedown', onDown)
        document.removeEventListener('keydown', onKey)
      }
    }
  }, [open])

  if (!active) return null

  function handleDelete() {
    if (!active) return
    if (!window.confirm(`Delete "${active.name}"? Its chat sessions will be removed.`)) return
    deleteProject(active.id)
    toast.success(`Project "${active.name}" deleted`)
    setOpen(false)
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="relative inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full pl-3 pr-2 py-1.5 text-xs text-text-1 transition-colors"
      >
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 bg-gradient-to-b from-accent to-accent2 rounded-full" />
        <span>📁</span>
        <span className="font-medium text-text-0 max-w-[200px] truncate">{active.name}</span>
        <motion.span animate={{ rotate: open ? 180 : 0 }} transition={{ duration: 0.18 }}>
          <ChevronDown size={14} className="text-text-2" />
        </motion.span>
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -8, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.98 }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            className="absolute top-full mt-2 left-0 bg-bg-1 border border-border-0 rounded-xl shadow-2xl w-80 overflow-hidden z-50 neon-glow-cyan"
          >
            <div className="max-h-72 overflow-auto">
              {projects.map((p) => {
                const isActive = p.id === activeId
                return (
                  <button
                    key={p.id}
                    onClick={() => {
                      setOpen(false)
                      switchProject(p.id)
                    }}
                    className={`relative w-full text-left px-3 py-2.5 text-xs flex items-center gap-2 transition-colors ${
                      isActive ? 'text-accent bg-accent/8' : 'text-text-1 hover:bg-bg-2 hover:text-text-0'
                    }`}
                  >
                    {isActive && <span className="absolute left-0 top-0 bottom-0 w-[2px] bg-accent" />}
                    <span>📁</span>
                    <span className="font-medium truncate">{p.name}</span>
                  </button>
                )
              })}
            </div>
            <div className="border-t border-border-0" />
            <button
              onClick={() => { setOpen(false); openWizard() }}
              className="w-full text-left px-3 py-2.5 text-xs text-accent hover:bg-accent/10 inline-flex items-center gap-2"
            >
              <Plus size={14} />
              <span className="font-medium">New project</span>
            </button>
            <button
              onClick={handleDelete}
              className="w-full text-left px-3 py-2.5 text-xs text-bad hover:bg-bad/10 inline-flex items-center gap-2"
            >
              <Trash2 size={14} />
              <span>Delete "{active.name}"</span>
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

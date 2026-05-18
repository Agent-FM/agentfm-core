import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { toast } from 'sonner'
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
        className="inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full px-3 py-1.5 text-xs text-text-1 transition-colors"
      >
        <span>📁</span>
        <span className="font-medium text-text-0 max-w-[200px] truncate">{active.name}</span>
        <span className="text-text-2">▾</span>
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.12 }}
            className="absolute top-full mt-1 left-0 bg-bg-1 border border-border-0 rounded-md shadow-xl w-72 max-h-80 overflow-auto z-50"
          >
            {projects.map((p) => (
              <button
                key={p.id}
                onClick={() => {
                  setOpen(false)
                  switchProject(p.id)
                }}
                className={`w-full text-left px-3 py-2 text-xs hover:bg-bg-2 ${
                  p.id === activeId ? 'text-accent' : 'text-text-1'
                }`}
              >
                <div className="flex items-center gap-2">
                  <span>📁</span>
                  <span className="font-medium truncate">{p.name}</span>
                </div>
              </button>
            ))}
            <div className="border-t border-border-0" />
            <button
              onClick={() => {
                setOpen(false)
                openWizard()
              }}
              className="w-full text-left px-3 py-2 text-xs text-text-1 hover:bg-bg-2 hover:text-accent"
            >
              + New project
            </button>
            <button
              onClick={handleDelete}
              className="w-full text-left px-3 py-2 text-xs text-rose-400 hover:bg-bg-2 hover:text-rose-300"
            >
              Delete "{active.name}"
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { toast } from 'sonner'
import { ChevronDown, Plus, Trash2, Folder, Globe, Lock } from 'lucide-react'
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
    return undefined
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
        className="relative inline-flex items-center gap-1.5 h-[22px] bg-raised border border-border-1 hover:bg-control rounded-ctl pl-2 pr-1.5 text-xs text-text-1 transition-colors"
      >
        <Folder size={14} strokeWidth={1.5} className="text-text-1" />
        <span className="font-medium text-text-0 max-w-[200px] truncate">{active.name}</span>
        <motion.span animate={{ rotate: open ? 180 : 0 }} transition={{ duration: 0.15, ease: 'easeOut' }}>
          <ChevronDown size={12} strokeWidth={1.5} className="text-text-2" />
        </motion.span>
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="absolute top-full mt-1.5 left-0 w-80 overflow-hidden rounded-card glass-strong shadow-float z-50"
          >
            <div className="px-3 pt-2 pb-1 text-2xs font-medium text-text-2">
              Projects
            </div>
            <div className="max-h-72 overflow-auto px-1 pb-1">
              {projects.map((p) => {
                const isActive = p.id === activeId
                const isPrivate = p.connectionMode === 'private'
                return (
                  <button
                    key={p.id}
                    onClick={() => {
                      setOpen(false)
                      switchProject(p.id)
                    }}
                    className={`relative w-full text-left px-2 py-1.5 rounded-ctl flex items-center gap-2 transition-colors duration-150 ${
                      isActive ? 'bg-accent/12 text-text-0' : 'text-text-1 hover:bg-white/[0.04] hover:text-text-0'
                    }`}
                  >
                    <span
                      className={`flex items-center justify-center w-6 h-6 rounded-ctl shrink-0 ${
                        isActive ? 'bg-accent/20 text-accent' : 'bg-bg-well text-text-2'
                      }`}
                    >
                      <Folder size={14} strokeWidth={1.5} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium truncate">{p.name}</span>
                      <span className="flex items-center gap-1 text-2xs text-text-2">
                        {isPrivate ? <Lock size={9} strokeWidth={1.5} /> : <Globe size={9} strokeWidth={1.5} />}
                        <span>{isPrivate ? 'Private swarm' : 'Public mesh'}</span>
                      </span>
                    </span>
                    {isActive && <span className="w-1.5 h-1.5 rounded-full bg-accent shrink-0" />}
                  </button>
                )
              })}
            </div>
            <div className="h-px bg-border-0" />
            <div className="p-1">
              <button
                onClick={() => { setOpen(false); openWizard() }}
                className="w-full text-left px-2 py-1.5 rounded-ctl text-sm text-accent hover:bg-accent/10 inline-flex items-center gap-2 transition-colors duration-150"
              >
                <span className="flex items-center justify-center w-6 h-6 rounded-ctl bg-accent/15 shrink-0">
                  <Plus size={14} strokeWidth={1.5} />
                </span>
                <span className="font-medium">New project</span>
              </button>
              <button
                onClick={handleDelete}
                className="w-full text-left px-2 py-1.5 rounded-ctl text-sm text-bad hover:bg-bad/10 inline-flex items-center gap-2 transition-colors duration-150"
              >
                <span className="flex items-center justify-center w-6 h-6 rounded-ctl bg-bad/12 shrink-0">
                  <Trash2 size={14} strokeWidth={1.5} />
                </span>
                <span className="truncate">Delete "{active.name}"</span>
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

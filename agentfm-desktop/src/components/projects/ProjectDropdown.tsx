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
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 bg-accent rounded-full" />
        <Folder size={14} className="text-text-2" />
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
            className="absolute top-full mt-2 left-0 w-80 overflow-hidden rounded-2xl border border-border-1 bg-bg-1 shadow-[0_24px_60px_-12px_rgba(0,0,0,.7)] z-50 ring-1 ring-black/40"
          >
            <div className="px-3 pt-3 pb-1.5 text-[10px] font-mono font-bold uppercase tracking-[0.16em] text-text-2">
              Projects
            </div>
            <div className="max-h-72 overflow-auto px-1.5 pb-1.5">
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
                    className={`relative w-full text-left px-2.5 py-2.5 rounded-xl flex items-center gap-2.5 transition-colors ${
                      isActive ? 'bg-accent/12 text-text-0' : 'text-text-1 hover:bg-bg-2 hover:text-text-0'
                    }`}
                  >
                    <span
                      className={`flex items-center justify-center w-7 h-7 rounded-lg shrink-0 ${
                        isActive ? 'bg-accent/20 text-accent' : 'bg-bg-2 text-text-2'
                      }`}
                    >
                      <Folder size={14} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-[13px] font-medium truncate">{p.name}</span>
                      <span className="flex items-center gap-1 text-[10px] text-text-2 mt-0.5">
                        {isPrivate ? <Lock size={9} /> : <Globe size={9} />}
                        <span className="uppercase tracking-wide">{isPrivate ? 'Private swarm' : 'Public mesh'}</span>
                      </span>
                    </span>
                    {isActive && <span className="w-1.5 h-1.5 rounded-full bg-accent shrink-0 shadow-[0_0_8px_#F7931E]" />}
                  </button>
                )
              })}
            </div>
            <div className="h-px bg-border-0" />
            <div className="p-1.5">
              <button
                onClick={() => { setOpen(false); openWizard() }}
                className="w-full text-left px-2.5 py-2.5 rounded-xl text-[13px] text-accent hover:bg-accent/10 inline-flex items-center gap-2.5 transition-colors"
              >
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-accent/15 shrink-0">
                  <Plus size={14} />
                </span>
                <span className="font-medium">New project</span>
              </button>
              <button
                onClick={handleDelete}
                className="w-full text-left px-2.5 py-2.5 rounded-xl text-[13px] text-bad hover:bg-bad/10 inline-flex items-center gap-2.5 transition-colors"
              >
                <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-bad/12 shrink-0">
                  <Trash2 size={14} />
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

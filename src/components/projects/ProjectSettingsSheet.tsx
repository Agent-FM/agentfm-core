import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError } from '../../lib/projectStore'
import type { ProjectColor } from '../../types/project'

const COLORS: { key: ProjectColor; hex: string }[] = [
  { key: 'emerald', hex: '#10b981' },
  { key: 'violet', hex: '#8b5cf6' },
  { key: 'rose', hex: '#f43f5e' },
  { key: 'cyan', hex: '#22d3ee' },
  { key: 'amber', hex: '#f59e0b' },
]

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

export function ProjectSettingsSheet() {
  const open = useUIStore((s) => s.isProjectSettingsOpen)
  const close = useUIStore((s) => s.closeProjectSettings)
  const active = useUIStore((s) => s.activeProject())
  const projects = useUIStore((s) => s.projects)
  const updateProject = useUIStore((s) => s.updateProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [icon, setIcon] = useState('🌐')
  const [color, setColor] = useState<ProjectColor>('emerald')
  const [relay, setRelay] = useState('')
  const [floor, setFloor] = useState(-0.5)

  useEffect(() => {
    if (!active) return
    setName(active.name)
    setIcon(active.icon)
    setColor(active.color)
    setRelay(active.relayMultiaddr ?? '')
    setFloor(active.reputationFloor)
  }, [active?.id, open])

  if (!active) return null

  async function save() {
    const relayValue = relay.trim() || null
    if (relayValue && !MULTIADDR_RE.test(relayValue)) {
      toast.error('That doesn’t look like a multiaddr')
      return
    }
    try {
      updateProject(active.id, {
        name,
        icon,
        color,
        relayMultiaddr: relayValue,
        reputationFloor: floor,
      })
    } catch (e) {
      const msg = e instanceof DuplicateRelayError ? e.message : (e as Error).message
      toast.error(msg)
      return
    }
    const meshChanged = relayValue !== active.relayMultiaddr || floor !== active.reputationFloor
    if (meshChanged) {
      setSwitching(true)
      try {
        await window.api.backend.restart({
          apiPort: useUIStore.getState().apiPort,
          reputationFloor: floor,
          relayMultiaddr: relayValue ?? undefined,
        })
      } catch (e) {
        toast.error('Restart failed: ' + (e as Error).message)
      } finally {
        setSwitching(false)
      }
    }
    toast.success('Project updated')
    close()
  }

  async function remove() {
    if (projects.length <= 1) {
      toast.error('You need at least one project')
      return
    }
    if (!window.confirm(`Delete "${active.name}"? Its chat sessions will be removed.`)) return
    deleteProject(active.id)
    close()
    toast.success('Project deleted')
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[65] flex justify-end"
          onClick={close}
        >
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[480px] h-full bg-bg-1 border-l border-border-0 overflow-auto p-6"
          >
            <div className="flex justify-between items-start mb-5">
              <h2 className="text-xl font-semibold text-text-0">Project settings</h2>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-4 mb-1.5">Icon &amp; color</label>
            <div className="flex items-center gap-3">
              <Input value={icon} onChange={(e) => setIcon(e.target.value.slice(0, 4))} className="w-16 text-center text-base" />
              <div className="flex gap-2">
                {COLORS.map((c) => (
                  <button
                    key={c.key}
                    onClick={() => setColor(c.key)}
                    className={`w-6 h-6 rounded-full transition-all ${
                      color === c.key ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
                    }`}
                    style={{ background: c.hex }}
                  />
                ))}
              </div>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">Relay multiaddr</label>
            <Input value={relay} onChange={(e) => setRelay(e.target.value)} placeholder="(blank = bundled public lighthouse)" className="font-mono text-2xs" />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">
              Reputation floor <span className="text-text-3 normal-case">({floor.toFixed(2)})</span>
            </label>
            <input type="range" min={-1} max={0} step={0.05} value={floor} onChange={(e) => setFloor(Number(e.target.value))} className="w-full accent-accent" />

            <div className="flex justify-end gap-2 mt-7">
              <Button onClick={close}>Cancel</Button>
              <Button variant="primary" onClick={save}>Save</Button>
            </div>

            <div className="mt-9 border-t border-border-0 pt-5">
              <div className="text-xs uppercase tracking-wider text-rose-400 mb-2">Danger zone</div>
              <Button onClick={remove}>Delete this project</Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

import { useState } from 'react'
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

export function CreateProjectWizard() {
  const open = useUIStore((s) => s.isCreateWizardOpen)
  const close = useUIStore((s) => s.closeCreateWizard)
  const addProject = useUIStore((s) => s.addProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [icon, setIcon] = useState('🌐')
  const [color, setColor] = useState<ProjectColor>('emerald')
  const [useDefault, setUseDefault] = useState(true)
  const [relay, setRelay] = useState('')
  const [floor, setFloor] = useState(-0.5)
  const [saving, setSaving] = useState(false)

  function reset() {
    setName('')
    setIcon('🌐')
    setColor('emerald')
    setUseDefault(true)
    setRelay('')
    setFloor(-0.5)
    setSaving(false)
  }

  async function create() {
    if (saving) return
    if (!name.trim()) {
      toast.error('Give the project a name')
      return
    }
    const relayValue = useDefault ? null : relay.trim()
    if (relayValue && !MULTIADDR_RE.test(relayValue)) {
      toast.error('That doesn’t look like a multiaddr')
      return
    }
    setSaving(true)
    let project
    try {
      project = addProject({
        name,
        icon,
        color,
        relayMultiaddr: relayValue,
        reputationFloor: floor,
      })
    } catch (e) {
      const msg = e instanceof DuplicateRelayError ? e.message : (e as Error).message
      toast.error(msg)
      setSaving(false)
      return
    }
    await window.api.settings.set('activeProjectId', project.id)
    useUIStore.setState({ activeProjectId: project.id })
    setSwitching(true)
    try {
      await window.api.backend.restart({
        apiPort: useUIStore.getState().apiPort,
        reputationFloor: project.reputationFloor,
        relayMultiaddr: project.relayMultiaddr ?? undefined,
      })
      toast.success(`Project "${project.name}" created`)
    } catch (e) {
      toast.error('Backend restart failed: ' + (e as Error).message)
    } finally {
      setSwitching(false)
      close()
      reset()
    }
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[70] flex items-center justify-center"
        >
          <motion.div
            initial={{ scale: 0.96, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.96, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 380, damping: 28 }}
            className="w-[520px] bg-bg-1 border border-border-0 rounded-xl p-7 shadow-2xl"
          >
            <div className="flex justify-between items-start mb-4">
              <div>
                <h2 className="text-xl font-semibold text-text-0">New project</h2>
                <p className="text-sm text-text-2 mt-1">
                  A project bundles a relay, a reputation threshold, and its own chats and starred agents.
                </p>
              </div>
              <button onClick={() => { close(); reset() }} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Team Mesh" autoFocus />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-4 mb-1.5">Icon &amp; color</label>
            <div className="flex items-center gap-3">
              <Input
                value={icon}
                onChange={(e) => setIcon(e.target.value.slice(0, 4))}
                className="w-16 text-center text-base"
              />
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

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">Relay</label>
            <label className="flex items-start gap-2 mb-2 cursor-pointer">
              <input type="radio" checked={useDefault} onChange={() => setUseDefault(true)} className="mt-1 accent-accent" />
              <div>
                <div className="text-sm text-text-0">Bundled public lighthouse</div>
                <div className="text-2xs text-text-2 mt-0.5">Recommended for first projects.</div>
              </div>
            </label>
            <label className="flex items-start gap-2 cursor-pointer">
              <input type="radio" checked={!useDefault} onChange={() => setUseDefault(false)} className="mt-1 accent-accent" />
              <div className="flex-1">
                <div className="text-sm text-text-0">Custom relay multiaddr</div>
                {!useDefault && (
                  <Input
                    className="mt-2 font-mono text-2xs"
                    placeholder="/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW…"
                    value={relay}
                    onChange={(e) => setRelay(e.target.value)}
                  />
                )}
              </div>
            </label>

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">
              Reputation floor <span className="text-text-3 normal-case">({floor.toFixed(2)})</span>
            </label>
            <input
              type="range"
              min={-1}
              max={0}
              step={0.05}
              value={floor}
              onChange={(e) => setFloor(Number(e.target.value))}
              className="w-full accent-accent"
            />
            <div className="flex justify-between text-2xs text-text-2 font-mono">
              <span>-1.0 (allow all)</span>
              <span>0.0 (strict)</span>
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <Button onClick={() => { close(); reset() }} disabled={saving}>Cancel</Button>
              <Button variant="primary" onClick={create} disabled={saving || !name.trim()}>
                {saving ? 'Creating…' : 'Create project'}
              </Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

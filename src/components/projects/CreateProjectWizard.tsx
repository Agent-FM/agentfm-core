import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError } from '../../lib/projectStore'
import { X, Zap } from 'lucide-react'

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

export function CreateProjectWizard() {
  const open = useUIStore((s) => s.isCreateWizardOpen)
  const close = useUIStore((s) => s.closeCreateWizard)
  const addProject = useUIStore((s) => s.addProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [useDefault, setUseDefault] = useState(true)
  const [relay, setRelay] = useState('')
  const [saving, setSaving] = useState(false)

  function reset() {
    setName('')
    setUseDefault(true)
    setRelay('')
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
      project = addProject({ name, relayMultiaddr: relayValue })
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
            className="w-[460px] bg-bg-1 border border-border-0 rounded-xl p-7 shadow-2xl neon-glow-cyan"
          >
            <div className="flex justify-between items-start mb-5">
              <div>
                <h2 className="text-xl font-semibold tracking-tight text-text-0">New project</h2>
                <p className="text-sm text-text-2 mt-1">
                  A project pairs a name with a relay. You can't change the relay later.
                </p>
              </div>
              <button onClick={() => { close(); reset() }} className="text-text-2 hover:text-text-0 text-lg"><X size={18} /></button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Team Mesh" autoFocus />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">Relay</label>
            <label className={`block border rounded-xl p-3 mb-2 cursor-pointer transition-all ${useDefault ? 'border-accent/40 bg-accent/8 neon-glow-cyan' : 'border-border-0 bg-bg-2 hover:border-border-1'}`}>
              <div className="flex items-start gap-2">
                <input type="radio" checked={useDefault} onChange={() => setUseDefault(true)} className="mt-1 accent-accent" />
                <div>
                  <div className="text-sm text-text-0">Bundled public lighthouse</div>
                  <div className="text-2xs text-text-2 mt-0.5">Recommended for first projects.</div>
                </div>
              </div>
            </label>
            <label className={`block border rounded-xl p-3 cursor-pointer transition-all ${!useDefault ? 'border-accent/40 bg-accent/8 neon-glow-cyan' : 'border-border-0 bg-bg-2 hover:border-border-1'}`}>
              <div className="flex items-start gap-2">
                <input type="radio" checked={!useDefault} onChange={() => setUseDefault(false)} className="mt-1 accent-accent" />
                <div className="flex-1">
                  <div className="text-sm text-text-0">Custom multiaddr</div>
                  {!useDefault && (
                    <Input
                      className="mt-2 font-mono text-2xs"
                      placeholder="/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW…"
                      value={relay}
                      onChange={(e) => setRelay(e.target.value)}
                    />
                  )}
                </div>
              </div>
            </label>

            <div className="flex justify-end gap-2 mt-7">
              <Button onClick={() => { close(); reset() }} disabled={saving}>Cancel</Button>
              <Button variant="primary" onClick={create} disabled={saving || !name.trim()}>
                <Zap size={12} />
                <span>{saving ? 'Creating…' : 'Create project'}</span>
              </Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

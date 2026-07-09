import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError, SWARM_KEY_HEX_RE } from '../../lib/projectStore'
import { X, Zap, Globe, Lock, Shuffle, Copy } from 'lucide-react'
import type { ConnectionMode } from '../../types/project'

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

function randomSwarmHex(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

export function CreateProjectWizard() {
  const open = useUIStore((s) => s.isCreateWizardOpen)
  const close = useUIStore((s) => s.closeCreateWizard)
  const addProject = useUIStore((s) => s.addProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [mode, setMode] = useState<ConnectionMode>('public')
  const [useDefault, setUseDefault] = useState(true)
  const [relay, setRelay] = useState('')
  const [swarmKey, setSwarmKey] = useState('')
  const [saving, setSaving] = useState(false)

  function reset() {
    setName('')
    setMode('public')
    setUseDefault(true)
    setRelay('')
    setSwarmKey('')
    setSaving(false)
  }

  function generateKey() {
    setSwarmKey(randomSwarmHex())
    toast.success('Generated swarm key, keep it secret')
  }

  async function copyKey() {
    if (!swarmKey) return
    try {
      await navigator.clipboard.writeText(swarmKey)
      toast.success('Swarm key copied')
    } catch {
      toast.error('Copy failed')
    }
  }

  async function create() {
    if (saving) return
    if (!name.trim()) {
      toast.error('Give the project a name')
      return
    }

    let relayValue: string | null
    if (mode === 'public') {
      relayValue = useDefault ? null : relay.trim() || null
    } else {
      relayValue = relay.trim()
      if (!relayValue) {
        toast.error('Private projects need a private relay multiaddr')
        return
      }
    }
    if (relayValue && !MULTIADDR_RE.test(relayValue)) {
      toast.error("That doesn't look like a multiaddr")
      return
    }

    const trimmedKey = swarmKey.trim().toLowerCase()
    if (mode === 'private' && !SWARM_KEY_HEX_RE.test(trimmedKey)) {
      toast.error('Swarm key must be 64 hex characters (256-bit PSK)')
      return
    }

    setSaving(true)
    let project
    try {
      project = addProject({
        name,
        relayMultiaddr: relayValue,
        connectionMode: mode,
        swarmKey: mode === 'private' ? trimmedKey : null,
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
        projectId: project.id,
        reputationFloor: project.reputationFloor,
        relayMultiaddr: project.relayMultiaddr ?? undefined,
        swarmKey:
          project.connectionMode === 'private' ? project.swarmKey ?? undefined : undefined,
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
          className="fixed inset-0 bg-bg-0/70 z-[70] flex items-center justify-center"
        >
          <motion.div
            initial={{ scale: 0.96, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.96, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 380, damping: 28 }}
            className="w-[520px] glass-strong rounded-sheet p-7 max-h-[90vh] overflow-y-auto"
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

            <label className="block text-2xs font-medium text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Team Mesh" aria-label="Project name" autoFocus />

            <label className="block text-2xs font-medium text-text-2 mt-5 mb-1.5">Connection mode</label>
            <div className="grid grid-cols-2 gap-2 mb-3">
              <button
                type="button"
                onClick={() => { setMode('public'); setUseDefault(true) }}
                className={`flex items-start gap-2 border rounded-card p-3 text-left transition-colors ${
                  mode === 'public'
                    ? 'border-accent/40 bg-accent/8'
                    : 'border-border-0 bg-bg-well hover:border-border-1'
                }`}
              >
                <Globe size={16} strokeWidth={1.5} className={mode === 'public' ? 'text-accent' : 'text-text-2'} />
                <div>
                  <div className="text-sm text-text-0">Public</div>
                  <div className="text-2xs text-text-2 mt-0.5">Join the open mesh.</div>
                </div>
              </button>
              <button
                type="button"
                onClick={() => { setMode('private'); setUseDefault(false) }}
                className={`flex items-start gap-2 border rounded-card p-3 text-left transition-colors ${
                  mode === 'private'
                    ? 'border-accent/40 bg-accent/8'
                    : 'border-border-0 bg-bg-well hover:border-border-1'
                }`}
              >
                <Lock size={16} strokeWidth={1.5} className={mode === 'private' ? 'text-accent' : 'text-text-2'} />
                <div>
                  <div className="text-sm text-text-0">Private</div>
                  <div className="text-2xs text-text-2 mt-0.5">PSK-gated swarm.</div>
                </div>
              </button>
            </div>

            {mode === 'public' ? (
              <>
                <label className="block text-2xs font-medium text-text-2 mb-1.5">Relay</label>
                <label className={`block border rounded-card p-3 mb-2 cursor-pointer transition-colors ${useDefault ? 'border-accent/40 bg-accent/8' : 'border-border-0 bg-bg-well hover:border-border-1'}`}>
                  <div className="flex items-start gap-2">
                    <input type="radio" checked={useDefault} onChange={() => setUseDefault(true)} className="mt-1 accent-accent" />
                    <div>
                      <div className="text-sm text-text-0">Bundled public lighthouse</div>
                      <div className="text-2xs text-text-2 mt-0.5">Recommended for first projects.</div>
                    </div>
                  </div>
                </label>
                <label className={`block border rounded-card p-3 cursor-pointer transition-colors ${!useDefault ? 'border-accent/40 bg-accent/8' : 'border-border-0 bg-bg-well hover:border-border-1'}`}>
                  <div className="flex items-start gap-2">
                    <input type="radio" checked={!useDefault} onChange={() => setUseDefault(false)} className="mt-1 accent-accent" />
                    <div className="flex-1">
                      <div className="text-sm text-text-0">Custom public multiaddr</div>
                      {!useDefault && (
                        <Input
                          className="mt-2 font-mono text-2xs"
                          aria-label="Public relay multiaddress"
                          placeholder="/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW…"
                          value={relay}
                          onChange={(e) => setRelay(e.target.value)}
                        />
                      )}
                    </div>
                  </div>
                </label>
              </>
            ) : (
              <>
                <label className="block text-2xs font-medium text-text-2 mb-1.5">
                  Private relay multiaddr
                </label>
                <Input
                  className="font-mono text-2xs"
                  aria-label="Private relay multiaddress"
                  placeholder="/ip4/10.0.0.42/tcp/4001/p2p/12D3KooW…"
                  value={relay}
                  onChange={(e) => setRelay(e.target.value)}
                />
                <p className="text-2xs text-text-2 mt-1">
                  Your private relay's address, the boss dials this on startup. Everyone in the
                  swarm must use the same one.
                </p>

                <label className="block text-2xs font-medium text-text-2 mt-5 mb-1.5">
                  Swarm key
                </label>
                <div className="flex gap-2">
                  <Input
                    className="font-mono text-2xs flex-1"
                    aria-label="Swarm key"
                    placeholder="64 hex chars (256-bit PSK)"
                    value={swarmKey}
                    onChange={(e) => setSwarmKey(e.target.value)}
                  />
                  <button
                    type="button"
                    onClick={generateKey}
                    title="Generate a fresh swarm key"
                    className="inline-flex items-center gap-1 px-2.5 rounded-ctl text-xs text-text-0 border border-accent/30 hover:border-accent/55 bg-accent/5 transition-colors"
                  >
                    <Shuffle size={12} /> Generate
                  </button>
                  <button
                    type="button"
                    onClick={copyKey}
                    disabled={!swarmKey}
                    title="Copy"
                    className="inline-flex items-center px-2.5 rounded-ctl text-xs text-text-1 glass-inset hover:text-text-0 hover:border-accent/40 disabled:opacity-40 transition-colors"
                  >
                    <Copy size={12} />
                  </button>
                </div>
                <p className="text-2xs text-text-2 mt-1">
                  Workers must hold this same 64-char hex key, or libp2p refuses every connection.
                </p>
              </>
            )}

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

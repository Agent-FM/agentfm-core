import { useState } from 'react'
import { motion } from 'framer-motion'
import { toast } from 'sonner'
import { useNavigate } from 'react-router-dom'
import { Copy, FileText, Trash2, Eye, EyeOff, Download, Lock, Globe, TriangleAlert } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { staggerItem } from '../lib/motion'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { Card } from '../components/primitives/Card'
import { Button } from '../components/primitives/Button'
import { Avatar } from '../components/primitives/Avatar'
import { StatusDot } from '../components/primitives/StatusDot'
import { LogsModal } from '../components/status/LogsModal'

export default function Settings() {
  const navigate = useNavigate()
  const active = useUIStore((s) => s.activeProject())
  const updateProject = useUIStore((s) => s.updateProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const { data: about } = useAbout()
  const backend = useBackend()
  const [showLogs, setShowLogs] = useState(false)
  const [revealKey, setRevealKey] = useState(false)

  if (!active) {
    return <div className="p-6 text-text-2">No active project. Create one to see settings.</div>
  }

  async function copy(value: string, label: string) {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(`${label} copied`)
    } catch {
      toast.error('Copy failed')
    }
  }

  function handleDelete() {
    if (!active) return
    const ok = window.confirm(`Delete "${active.name}"? Sessions are removed; artifacts on disk are preserved.`)
    if (!ok) return
    const name = active.name
    deleteProject(active.id)
    toast.success(`Project "${name}" deleted`)
    navigate('/radar')
  }

  function setThreshold(value: number) {
    if (!active) return
    const clamped = Math.max(-1, Math.min(0, Number(value.toFixed(2))))
    updateProject(active.id, { reputationFloor: clamped })
  }

  const isPrivate = active.connectionMode === 'private'

  return (
    <>
      <div className="p-4 max-w-3xl">
        {/* Brand */}
        <div className="mb-5 flex items-baseline gap-3">
          <h1 className="text-lg font-semibold leading-none text-text-0">AgentFM Settings</h1>
          <div className="text-sm text-text-1">
            <span className="text-text-0">{active.name}</span>
          </div>
        </div>

        {/* BACKEND */}
        <motion.section className="mb-4" {...staggerItem(0)}>
          <SectionLabel as="h2">Backend</SectionLabel>
          <Card className="glass p-3 mt-1.5">
            <div className="flex items-center gap-2.5 text-sm">
              <StatusDot tone={backend.ok ? 'ok' : 'bad'} pulse={backend.ok} />
              <span className="font-semibold text-text-0">{backend.ok ? 'Healthy' : 'Down'}</span>
              <span className="font-mono text-text-2 text-xs tabular-nums">
                {backend.online_workers} online worker{backend.online_workers === 1 ? '' : 's'}
              </span>
              <span className="ml-auto font-mono text-2xs tabular-nums px-1.5 py-0.5 rounded-[4px] text-accent bg-accent/10 border border-accent/25">
                v{about?.version ?? '…'}
              </span>
            </div>
            <div className="mt-3">
              <Button onClick={() => setShowLogs(true)}>
                <FileText size={12} />
                <span>View logs</span>
              </Button>
            </div>
          </Card>
        </motion.section>

        {/* CONNECTION */}
        <motion.section className="mb-4" {...staggerItem(1)}>
          <SectionLabel as="h2">Connection</SectionLabel>
          <Card className="glass p-3 mt-1.5">
            <div className="flex items-center gap-2.5 mb-3">
              <Avatar size="sm">
                {isPrivate ? (
                  <Lock size={14} strokeWidth={1.5} />
                ) : (
                  <Globe size={14} strokeWidth={1.5} />
                )}
              </Avatar>
              <div>
                <div className="text-sm font-semibold text-text-0">
                  {isPrivate ? 'Private' : 'Public'}{' '}
                  <span className="text-xs text-text-2 font-normal">
                    {isPrivate ? 'PSK-isolated' : 'public lighthouse'}
                  </span>
                </div>
                <div className="text-xs text-text-2 font-mono mt-0.5">
                  {isPrivate ? 'refuses any peer without the same swarm key' : 'joins the public mesh'}
                </div>
              </div>
            </div>
            <div className="pt-3 border-t border-border-0 space-y-2">
              <TechRow
                k="Relay multiaddr"
                v={active.relayMultiaddr || 'Bundled public lighthouse'}
                onCopy={active.relayMultiaddr ? () => copy(active.relayMultiaddr!, 'Multiaddr') : undefined}
              />
              {isPrivate && active.swarmKey && (
                <div className="flex items-center gap-3">
                  <div className="w-[130px] flex-shrink-0 text-2xs font-medium text-text-2">Swarm key</div>
                  <div className="flex-1 font-mono text-xs text-text-1 break-all">
                    {revealKey ? active.swarmKey : '•'.repeat(64)}
                  </div>
                  <IconBtn title={revealKey ? 'Hide' : 'Reveal'} onClick={() => setRevealKey(r => !r)}>
                    {revealKey ? <EyeOff size={12} /> : <Eye size={12} />}
                  </IconBtn>
                  <IconBtn title="Copy" onClick={() => copy(active.swarmKey!, 'Swarm key')}>
                    <Copy size={12} />
                  </IconBtn>
                  <IconBtn
                    title="Export .key"
                    onClick={async () => {
                      const res = await window.api.app.saveSwarmKeyFile(active.swarmKey!, `${active.name}.swarm.key`)
                      if (res.ok && res.path) toast.success(`Exported to ${res.path}`)
                    }}
                  ><Download size={12} /></IconBtn>
                </div>
              )}
            </div>
            {isPrivate && (
              <div className="text-2xs text-text-2 mt-3 inline-flex items-center gap-1.5">
                <TriangleAlert size={12} strokeWidth={1.5} />
                Changing the swarm key requires restarting the boss.
              </div>
            )}
          </Card>
        </motion.section>

        {/* THRESHOLD */}
        <motion.section className="mb-4" {...staggerItem(2)}>
          <SectionLabel as="h2">Auto-refuse threshold</SectionLabel>
          <Card className="glass p-3 mt-1.5">
            <div className="text-sm text-text-1 leading-[1.5] mb-3">
              Workers with rating below{' '}
              <span className="font-mono font-semibold text-xs tabular-nums text-accent">
                {active.reputationFloor.toFixed(2)}
              </span>{' '}
              are auto-refused. Equivocators are always blocked.
            </div>
            <input
              type="range" min={-1} max={0} step={0.05} value={active.reputationFloor}
              aria-label="Auto-refuse reputation threshold"
              onChange={(e) => setThreshold(parseFloat(e.target.value))}
              className="w-full h-[3px] accent-accent focus:outline-none"
            />
            <div className="flex justify-between text-2xs text-text-2 font-mono tabular-nums mt-2">
              <span>−1.00 strict</span>
              <span>0.00 permissive</span>
            </div>
          </Card>
        </motion.section>

        {/* THIS BOSS */}
        <motion.section className="mb-4" {...staggerItem(3)}>
          <SectionLabel as="h2">This boss</SectionLabel>
          <Card className="glass p-3 mt-1.5 space-y-2">
            <TechRow k="Peer ID" v={about?.boss_peer_id ?? '…'} valueClass="text-accent"
                     onCopy={() => copy(about?.boss_peer_id ?? '', 'Peer ID')} />
            <TechRow k="Relay peer ID" v={about?.relay_peer_id || 'Not connected'} />
            <TechRow k="Backend version" v={about?.version ?? '…'} />
            <TechRow k="Reputation floor" v={active.reputationFloor.toFixed(2)} />
            <TechRow k="Ledger storage" v="~/.agentfm/" />
          </Card>
        </motion.section>

        {/* DANGER */}
        <motion.section className="mb-4" {...staggerItem(4)}>
          <SectionLabel as="h2" tone="bad">Danger zone</SectionLabel>
          <Card className="glass p-3 mt-1.5 flex items-center gap-4">
            <div className="flex-1">
              <div className="text-sm font-semibold text-bad">Delete this project</div>
              <div className="text-sm text-text-1 mt-0.5">
                Removes the project entry and chat sessions. Artifacts on disk stay.
              </div>
            </div>
            <Button variant="danger" onClick={handleDelete}>
              <Trash2 size={13} />
              <span>Delete project</span>
            </Button>
          </Card>
        </motion.section>
      </div>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

function TechRow({ k, v, onCopy, valueClass = 'text-text-1' }:
  { k: string; v: string; onCopy?: () => void; valueClass?: string }) {
  return (
    <div className="flex items-baseline gap-3 text-sm">
      <div className="w-[130px] flex-shrink-0 text-2xs font-medium text-text-2">{k}</div>
      <div className={`flex-1 font-mono text-xs tabular-nums break-all ${valueClass}`}>{v}</div>
      {onCopy && (
        <button onClick={onCopy} className="text-text-2 hover:text-accent" title="Copy">
          <Copy size={12} />
        </button>
      )}
    </div>
  )
}

function IconBtn({ title, onClick, children }:
  { title: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} title={title}
      className="w-[22px] h-[22px] rounded-ctl bg-control hover:bg-control-hover border border-border-1
        text-text-1 hover:text-text-0 inline-flex items-center justify-center transition-colors active:scale-[0.98]">
      {children}
    </button>
  )
}

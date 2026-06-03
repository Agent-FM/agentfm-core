import { useState } from 'react'
import { toast } from 'sonner'
import { useNavigate } from 'react-router-dom'
import { Copy, FileText, Trash2, Eye, EyeOff, Download } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { NeonCard } from '../components/primitives/NeonCard'
import { Avatar } from '../components/primitives/Avatar'
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
    return <div className="p-7 text-text-2">No active project. Create one to see settings.</div>
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
      <div className="p-7 max-w-3xl">
        {/* Brand */}
        <div className="mb-9 flex items-baseline gap-4">
          <div className="text-[44px] font-bold tracking-[-0.025em] leading-none">
            Agent<span
              className="animate-text-shimmer"
              style={{
                background: 'linear-gradient(120deg,#22d3ee,#a855f7,#22d3ee)',
                backgroundSize: '200% 100%',
                WebkitBackgroundClip: 'text', backgroundClip: 'text', color: 'transparent',
              }}
            >FM</span>
          </div>
          <div className="text-[15px] text-text-1">
            Settings for <span className="text-text-0 font-semibold">{active.name}</span>
          </div>
        </div>

        {/* BACKEND */}
        <section className="mb-7">
          <SectionLabel>BACKEND</SectionLabel>
          <NeonCard className="p-5">
            <div className="flex items-center gap-3 text-[15px]">
              <span
                className="w-2 h-2 rounded-full animate-pulse-cyan"
                style={{
                  background: backend.ok ? '#22d3ee' : '#f43f5e',
                  boxShadow: `0 0 8px ${backend.ok ? '#22d3ee' : '#f43f5e'}`,
                }}
              />
              <span className="font-semibold text-text-0">{backend.ok ? 'Healthy' : 'Down'}</span>
              <span className="font-mono text-text-2 text-[13px]">
                {backend.online_workers} online worker{backend.online_workers === 1 ? '' : 's'}
              </span>
              <span
                className="ml-auto font-mono text-[12px] px-2.5 py-1 rounded-full
                  text-accent2-light bg-accent2/[.08] border border-accent2/[.25]"
              >v{about?.version ?? '…'}</span>
            </div>
            <button
              onClick={() => setShowLogs(true)}
              className="mt-4 inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-[13px]
                font-semibold text-text-0 border border-accent/30 hover:border-accent/55
                hover:shadow-[0_0_14px_-4px_rgba(34,211,238,.4)] transition-all"
            ><FileText size={12} /><span>View logs</span></button>
          </NeonCard>
        </section>

        {/* CONNECTION */}
        <section className="mb-7">
          <SectionLabel>CONNECTION</SectionLabel>
          <NeonCard className="p-5">
            <div className="flex items-center gap-3.5 mb-4">
              <Avatar size="md" emoji={isPrivate ? '🔒' : '🌐'} />
              <div>
                <div className="text-[20px] font-semibold tracking-[-0.01em]">
                  {isPrivate ? 'Private' : 'Public'}{' '}
                  <span className="text-[14px] text-accent2-light font-normal">
                    {isPrivate ? '(PSK-isolated)' : '(public lighthouse)'}
                  </span>
                </div>
                <div className="text-[13px] text-text-2 font-mono mt-0.5">
                  {isPrivate ? 'refuses any peer without the same swarm key' : 'joins the public mesh'}
                </div>
              </div>
            </div>
            <div className="pt-4 border-t border-accent/10 space-y-3">
              <TechRow
                k="Relay multiaddr"
                v={active.relayMultiaddr || '(bundled public lighthouse)'}
                onCopy={active.relayMultiaddr ? () => copy(active.relayMultiaddr!, 'Multiaddr') : undefined}
              />
              {isPrivate && active.swarmKey && (
                <div className="flex items-center gap-3.5">
                  <div className="w-[130px] flex-shrink-0 font-mono uppercase font-bold text-text-2"
                       style={{fontSize:11,letterSpacing:'0.14em'}}>Swarm key</div>
                  <div className="flex-1 font-mono text-[13px] text-accent2-light break-all">
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
              <div className="text-[11px] text-text-2 mt-3">
                ⚠ Changing the swarm key requires restarting the boss.
              </div>
            )}
          </NeonCard>
        </section>

        {/* THRESHOLD */}
        <section className="mb-7">
          <SectionLabel>AUTO-REFUSE THRESHOLD</SectionLabel>
          <NeonCard className="p-5">
            <div className="text-[15px] text-text-1 leading-[1.5] mb-4">
              Workers with rating below{' '}
              <span
                className="font-mono font-bold text-[18px]"
                style={{
                  background: 'linear-gradient(135deg,#a5f3fc,#d8b4fe)',
                  WebkitBackgroundClip: 'text', backgroundClip: 'text', color: 'transparent',
                  textShadow: '0 0 8px rgba(168,85,247,.4)',
                }}
              >{active.reputationFloor.toFixed(2)}</span>{' '}
              are auto-refused. Equivocators are always blocked.
            </div>
            <input
              type="range" min={-1} max={0} step={0.05} value={active.reputationFloor}
              onChange={(e) => setThreshold(parseFloat(e.target.value))}
              className="w-full accent-cyan-400"
            />
            <div className="flex justify-between text-[11px] text-text-3 font-mono mt-2">
              <span>−1.00 strict</span>
              <span>0.00 permissive</span>
            </div>
          </NeonCard>
        </section>

        {/* THIS BOSS */}
        <section className="mb-7">
          <SectionLabel>THIS BOSS</SectionLabel>
          <NeonCard className="p-5 space-y-3">
            <TechRow k="Peer ID" v={about?.boss_peer_id ?? '…'} valueClass="text-accent-high"
                     onCopy={() => copy(about?.boss_peer_id ?? '', 'Peer ID')} />
            <TechRow k="Relay peer ID" v={about?.relay_peer_id || '(not connected)'} />
            <TechRow k="Backend version" v={about?.version ?? '…'} />
            <TechRow k="Reputation floor" v={active.reputationFloor.toFixed(2)} />
            <TechRow k="Ledger storage" v="~/.agentfm/" />
          </NeonCard>
        </section>

        {/* DANGER */}
        <section className="mb-7">
          <SectionLabel tone="rose">DANGER ZONE</SectionLabel>
          <div className="rounded-2xl p-5 flex items-center gap-4"
               style={{
                 background:'linear-gradient(135deg,rgba(244,63,94,.08),rgba(244,63,94,.02))',
                 border:'1px solid rgba(244,63,94,.3)',
               }}>
            <div className="flex-1">
              <div className="text-[16px] font-semibold text-bad">Delete this project</div>
              <div className="text-[13px] text-text-1 mt-1">
                Removes the project entry and chat sessions. Artifacts on disk stay.
              </div>
            </div>
            <button
              onClick={handleDelete}
              className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl
                font-bold text-[13px] text-white border-0
                bg-gradient-to-br from-bad to-rose-700
                shadow-[0_4px_14px_-4px_rgba(244,63,94,.5)] hover:brightness-110 transition-all"
            ><Trash2 size={13} /><span>Delete project</span></button>
          </div>
        </section>
      </div>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

function TechRow({ k, v, onCopy, valueClass = 'text-text-1' }:
  { k: string; v: string; onCopy?: () => void; valueClass?: string }) {
  return (
    <div className="flex items-baseline gap-3.5 text-[14px]">
      <div className="w-[130px] flex-shrink-0 font-mono uppercase font-bold text-text-2"
           style={{fontSize:11,letterSpacing:'0.14em'}}>{k}</div>
      <div className={`flex-1 font-mono text-[13px] break-all ${valueClass}`}>{v}</div>
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
      className="w-7 h-7 rounded-md border border-accent/25 hover:border-accent/45
        text-accent inline-flex items-center justify-center transition-colors">
      {children}
    </button>
  )
}

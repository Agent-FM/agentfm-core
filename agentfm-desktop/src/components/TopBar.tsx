import { useState } from 'react'
import { toast } from 'sonner'
import { Copy, Check } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { ProjectDropdown } from './projects/ProjectDropdown'
import { RelayPill } from './primitives/RelayPill'
import { useAbout } from '../lib/query'
import logoUrl from '../assets/logo-ver-2.png'

function truncateMultiaddr(m: string): string {
  if (m.length <= 32) return m
  return m.slice(0, 14) + '…' + m.slice(-14)
}

const isMac = typeof window !== 'undefined' && window.api?.platform === 'darwin'
const noDrag = { WebkitAppRegion: 'no-drag' as const }

export function TopBar() {
  const active = useUIStore((s) => s.activeProject())
  const { data: about } = useAbout()
  const mode = active?.connectionMode ?? 'public'
  const [copied, setCopied] = useState(false)

  async function copyRelay() {
    if (!active) return
    const value = active.relayMultiaddr ?? about?.relay_multiaddr ?? '(bundled public lighthouse)'
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 800)
      toast.success('Relay copied')
    } catch {
      toast.error('Could not copy')
    }
  }

  return (
    <header
      className={`glass-bar h-14 flex items-center gap-4 px-4 select-none relative z-50 ${isMac ? 'pl-[92px]' : ''}`}
      style={{ WebkitAppRegion: 'drag' }}
    >
      <span
        className="absolute inset-x-0 bottom-0 h-px pointer-events-none"
        style={{
          background:
            'linear-gradient(90deg, rgba(247,147,30,.45) 0%, rgba(247,147,30,0) 100%)',
        }}
      />
      <img
        src={logoUrl}
        alt="AgentFM"
        className="h-7 w-auto object-contain shrink-0 select-none"
        draggable={false}
      />
      {active && (
        <div className="shrink-0" style={noDrag}>
          <ProjectDropdown />
        </div>
      )}
      <div className="flex-1 min-w-[12px]" />
      {about?.relay_peer_id && (
        <div className="shrink-0" style={noDrag}>
          <RelayPill peerId={about.relay_peer_id} mode={mode} />
        </div>
      )}
      {active && (
        <button
          onClick={copyRelay}
          className="group inline-flex items-center gap-1.5 min-w-0 shrink text-2xs text-text-2 hover:text-text-0 font-mono transition-colors px-2 py-1 rounded"
          title={active.relayMultiaddr ?? about?.relay_multiaddr ?? 'bundled public lighthouse'}
          style={noDrag}
        >
          <span className="truncate">relay: {active.relayMultiaddr ? truncateMultiaddr(active.relayMultiaddr) : 'bundled'}</span>
          {copied ? <Check size={12} className="text-accent" /> : <Copy size={12} className="opacity-0 group-hover:opacity-70 transition-opacity" />}
        </button>
      )}
    </header>
  )
}

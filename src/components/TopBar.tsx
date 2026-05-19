import { useState } from 'react'
import { toast } from 'sonner'
import { Copy, Check } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { ProjectDropdown } from './projects/ProjectDropdown'

function truncateMultiaddr(m: string): string {
  if (m.length <= 32) return m
  return m.slice(0, 14) + '…' + m.slice(-14)
}

export function TopBar() {
  const active = useUIStore((s) => s.activeProject())
  const [copied, setCopied] = useState(false)

  async function copyRelay() {
    if (!active) return
    const value = active.relayMultiaddr ?? '(bundled public lighthouse)'
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
      className="h-12 bg-bg-0 flex items-center gap-4 px-4 select-none relative"
      style={{
        borderBottom: '1px solid transparent',
        backgroundImage:
          'linear-gradient(#07090d,#07090d), linear-gradient(90deg, rgba(34,211,238,.35) 0%, rgba(34,211,238,0) 50%, rgba(168,85,247,.25) 100%)',
        backgroundOrigin: 'border-box',
        backgroundClip: 'padding-box, border-box',
      }}
    >
      <div className="text-sm font-semibold tracking-tight">
        Agent<span className="text-accent glow-text-cyan">FM</span>
      </div>
      {active && <ProjectDropdown />}
      <div className="flex-1" />
      {active && (
        <button
          onClick={copyRelay}
          className="group inline-flex items-center gap-1.5 text-2xs text-text-2 hover:text-text-0 font-mono transition-colors px-2 py-1 rounded"
          title={active.relayMultiaddr ?? 'bundled public lighthouse'}
        >
          <span>relay: {active.relayMultiaddr ? truncateMultiaddr(active.relayMultiaddr) : 'bundled'}</span>
          {copied ? <Check size={12} className="text-accent" /> : <Copy size={12} className="opacity-0 group-hover:opacity-70 transition-opacity" />}
        </button>
      )}
    </header>
  )
}

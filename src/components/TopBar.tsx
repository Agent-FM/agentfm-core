import { toast } from 'sonner'
import { useUIStore } from '../lib/store'
import { ProjectDropdown } from './projects/ProjectDropdown'

function truncateMultiaddr(m: string): string {
  if (m.length <= 32) return m
  return m.slice(0, 14) + '…' + m.slice(-14)
}

export function TopBar() {
  const active = useUIStore((s) => s.activeProject())

  async function copyRelay() {
    if (!active) return
    const value = active.relayMultiaddr ?? '(bundled public lighthouse)'
    try {
      await navigator.clipboard.writeText(value)
      toast.success('Relay copied')
    } catch {
      toast.error('Could not copy')
    }
  }

  return (
    <header className="h-11 border-b border-border-0 bg-bg-0 flex items-center gap-4 px-3 select-none">
      <div className="text-sm font-semibold tracking-tight text-text-0">AgentFM</div>
      {active && <ProjectDropdown />}
      <div className="flex-1" />
      {active && (
        <button
          onClick={copyRelay}
          className="text-2xs text-text-2 hover:text-text-0 font-mono transition-colors"
          title={active.relayMultiaddr ?? 'bundled public lighthouse'}
        >
          relay: {active.relayMultiaddr ? truncateMultiaddr(active.relayMultiaddr) : 'bundled'}
        </button>
      )}
    </header>
  )
}

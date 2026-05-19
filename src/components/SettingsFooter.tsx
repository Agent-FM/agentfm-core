import { Settings as SettingsIcon } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { useBackend } from '../hooks/useBackend'
import { StatusDot } from './primitives/StatusDot'

export function SettingsFooter() {
  const openSettings = useUIStore((s) => s.openSettingsSheet)
  const backend = useBackend()
  const tone = backend.ok ? 'cyan' : 'rose'

  return (
    <footer className="border-t border-border-0 bg-bg-0 px-3 py-2 flex items-center gap-3">
      <button
        onClick={openSettings}
        className="inline-flex items-center gap-2 text-xs text-text-2 hover:text-text-0 transition-colors px-2 py-1 rounded-md hover:bg-bg-1"
      >
        <SettingsIcon size={14} />
        <span>Settings</span>
      </button>
      <div className="flex-1" />
      <div className="inline-flex items-center gap-1.5 text-2xs text-text-2 font-mono">
        <StatusDot tone={tone} pulse={backend.ok} size="sm" />
        <span>backend {backend.ok ? 'healthy' : 'down'}</span>
      </div>
    </footer>
  )
}

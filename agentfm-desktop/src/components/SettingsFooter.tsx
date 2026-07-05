import { useBackend } from '../hooks/useBackend'
import { StatusDot } from './primitives/StatusDot'

export function SettingsFooter() {
  const backend = useBackend()
  const tone = backend.ok ? 'cyan' : 'rose'

  return (
    <footer className="border-t border-border-0 bg-bg-0 px-3 py-2 flex items-center gap-3">
      <div className="flex-1" />
      <div className="inline-flex items-center gap-1.5 text-2xs text-text-2 font-mono">
        <StatusDot tone={tone} pulse={backend.ok} size="sm" />
        <span>backend {backend.ok ? 'healthy' : 'down'}</span>
      </div>
    </footer>
  )
}

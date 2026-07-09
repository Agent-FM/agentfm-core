import { useBackend } from '../hooks/useBackend'
import { StatusDot } from './primitives/StatusDot'

export function SettingsFooter() {
  const backend = useBackend()
  const tone = backend.ok ? 'accent' : 'bad'

  return (
    <footer className="bg-chrome border-t border-border-0 h-[22px] px-3 flex items-center gap-3 relative z-40">
      <div className="flex-1" />
      <div className="inline-flex items-center gap-1.5 text-2xs text-text-2 font-mono">
        <StatusDot tone={tone} pulse={backend.ok} size="sm" />
        <span>backend {backend.ok ? 'healthy' : 'down'}</span>
      </div>
    </footer>
  )
}

import { useUIStore } from '../../lib/store'

const COLOR_HEX: Record<string, string> = {
  emerald: '#10b981',
  violet: '#8b5cf6',
  rose: '#f43f5e',
  cyan: '#22d3ee',
  amber: '#f59e0b',
}

export function ProjectPill() {
  const active = useUIStore((s) => s.activeProject())
  const open = useUIStore((s) => s.openProjectSettings)
  if (!active) return <div className="w-32" />

  return (
    <button
      onClick={open}
      className="inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full px-3 py-1.5 text-xs text-text-1 transition-colors"
      title="Project settings"
    >
      <span>{active.icon}</span>
      <span className="font-medium text-text-0 max-w-[180px] truncate">{active.name}</span>
      <span
        className="w-1.5 h-1.5 rounded-full"
        style={{ background: COLOR_HEX[active.color] ?? '#10b981' }}
      />
    </button>
  )
}

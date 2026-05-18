import { useUIStore } from '../lib/store'

export function SettingsFooter() {
  const openSettings = useUIStore((s) => s.openSettingsSheet)
  return (
    <footer className="border-t border-border-0 bg-bg-0 px-3 py-2 flex items-center">
      <button
        onClick={openSettings}
        className="inline-flex items-center gap-2 text-xs text-text-2 hover:text-text-0 transition-colors px-2 py-1"
      >
        <span className="text-sm leading-none">⚙</span>
        <span>Settings</span>
      </button>
    </footer>
  )
}

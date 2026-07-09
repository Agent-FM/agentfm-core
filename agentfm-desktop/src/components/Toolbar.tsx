import { PanelLeft, PanelBottom, PanelRight, TriangleAlert, Copy } from 'lucide-react'
import { toast } from 'sonner'
import { shortenPeerID } from '../lib/peer'
import { useUIStore } from '../lib/store'
import { useWorkers, useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { ProjectDropdown } from './projects/ProjectDropdown'

const isMac = typeof window !== 'undefined' && window.api?.platform === 'darwin'
const noDrag = { WebkitAppRegion: 'no-drag' as const }

interface Props {
  showNavigator: boolean
  showDebug: boolean
  showInspector: boolean
  onToggleNavigator: () => void
  onToggleDebug: () => void
  onToggleInspector: () => void
}

function PaneToggle({ active, onClick, label, children }: {
  active: boolean; onClick: () => void; label: string; children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      aria-pressed={active}
      title={label}
      style={noDrag}
      className={`p-1 rounded-ctl transition-colors duration-150 cursor-pointer ${
        active ? 'text-accent' : 'text-text-1 hover:text-text-0'
      }`}
    >
      {children}
    </button>
  )
}

export function Toolbar(props: Props) {
  const active = useUIStore((s) => s.activeProject())
  const backend = useBackend()
  const { data: about } = useAbout()
  const { data: workersData, isLoading } = useWorkers(false)

  const agents = workersData?.agents ?? []
  const online = agents.filter((a) => a.online !== false).length
  const busy = agents.filter((a) => (a.current_tasks ?? 0) > 0).length
  const refused = agents.filter((a) => a.dispatch_allowed === false).length

  const relayLine = !backend.ok
    ? 'Backend down'
    : about?.relay_peer_id
      ? 'Connected to relay'
      : 'Connecting to relay…'
  const relayId = about?.relay_peer_id

  return (
    <header
      className={`glass-bar h-[52px] shrink-0 flex items-center gap-3 pr-3 border-b border-border-0 select-none relative z-50 ${
        isMac ? 'pl-[78px]' : 'pl-3'
      }`}
      style={{ WebkitAppRegion: 'drag' }}
    >
      {active && (
        <div className="shrink-0" style={noDrag}>
          <ProjectDropdown />
        </div>
      )}

      <div className="flex-1 min-w-[8px]" />

      {/* Activity Viewer pill */}
      <div
        className="relative overflow-hidden w-[420px] max-w-[40vw] h-[30px] shrink rounded-card bg-raised border border-border-0 px-3 flex items-center gap-2"
        style={noDrag}
      >
        <div className="min-w-0 flex-1 leading-tight">
          <div className="flex items-center gap-1.5 text-2xs text-text-0 min-w-0">
            <span className="shrink-0">{relayLine}</span>
            {relayId && (
              <button
                onClick={() =>
                  navigator.clipboard
                    .writeText(relayId)
                    .then(() => toast.success('Relay address copied'))
                    .catch(() => toast.error('Copy failed'))
                }
                title={`Copy relay address ${relayId}`}
                style={noDrag}
                className="group/relay inline-flex items-center gap-1 min-w-0 font-mono text-text-2 hover:text-text-0 transition-colors cursor-pointer"
              >
                <span className="truncate">{shortenPeerID(relayId, 6, 5)}</span>
                <Copy size={10} className="opacity-0 group-hover/relay:opacity-70 transition-opacity shrink-0" />
              </button>
            )}
          </div>
          <div className="flex items-center gap-2.5 text-2xs text-text-1 truncate tnum">
            <span>{online} online</span>
            <span>{busy} busy</span>
            <span>{active ? (active.connectionMode === 'private' ? 'private swarm' : 'public mesh') : 'no project'}</span>
          </div>
        </div>
        {refused > 0 && (
          <span className="shrink-0 inline-flex items-center gap-0.5 text-2xs text-warn tnum" title={`${refused} worker(s) below reputation floor`}>
            <TriangleAlert size={10} strokeWidth={2} />
            {refused}
          </span>
        )}
        {isLoading && (
          <span className="absolute bottom-0 left-0 right-0 h-[2px] overflow-hidden">
            <span className="block h-full w-1/3 bg-accent animate-progress-slide" />
          </span>
        )}
      </div>

      <div className="flex-1 min-w-[8px]" />

      {/* Pane toggles */}
      <div className="flex items-center gap-0.5 shrink-0" style={noDrag}>
        <PaneToggle active={props.showNavigator} onClick={props.onToggleNavigator} label="Toggle navigator">
          <PanelLeft size={17} strokeWidth={1.5} />
        </PaneToggle>
        <PaneToggle active={props.showDebug} onClick={props.onToggleDebug} label="Toggle debug area">
          <PanelBottom size={17} strokeWidth={1.5} />
        </PaneToggle>
        <PaneToggle active={props.showInspector} onClick={props.onToggleInspector} label="Toggle inspector">
          <PanelRight size={17} strokeWidth={1.5} />
        </PaneToggle>
      </div>
    </header>
  )
}

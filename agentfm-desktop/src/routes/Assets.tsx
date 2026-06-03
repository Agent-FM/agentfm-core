import { useEffect, useState, useCallback, useMemo } from 'react'
import { toast } from 'sonner'
import { ExternalLink, RefreshCw } from 'lucide-react'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { GradientButton } from '../components/primitives/GradientButton'
import { usePeerIdentityCache } from '../lib/peerIdentityCache'
import { shortenPeerID } from '../lib/peer'
import type { ArtifactListEntry } from '../../electron/preload'

function formatBytes(b: number): string {
  if (b < 1024) return `${b} B`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1024 * 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(1)} MB`
  return `${(b / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatAge(ms: number): string {
  const diff = Date.now() - ms
  if (diff < 60_000) return 'just now'
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)}h ago`
  return `${Math.floor(diff / 86400_000)}d ago`
}

export default function Assets() {
  const [items, setItems] = useState<ArtifactListEntry[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [activeProject, setActiveProject] = useState<string>('all')
  const [filter, setFilter] = useState('')
  const peerCache = usePeerIdentityCache((s) => s.byPeerId)

  const resolveIdentity = useCallback(
    (it: ArtifactListEntry): { name: string; description?: string; peerLabel?: string } => {
      const meta = it.metadata
      const cached = meta?.agentPeerId ? peerCache[meta.agentPeerId] : undefined
      const name =
        (meta?.agentName?.trim() || cached?.name?.trim() || cached?.agent_capability?.trim()) ?? ''
      const description = meta?.agentDescription?.trim() || cached?.description?.trim()
      const peerLabel = meta?.agentPeerId ? shortenPeerID(meta.agentPeerId, 6, 5) : undefined
      return { name, description, peerLabel }
    },
    [peerCache],
  )

  const refresh = useCallback(async () => {
    try {
      const list = await window.api.app.listArtifacts()
      setItems(list); setError(null)
    } catch (e) { setError((e as Error).message) }
  }, [])
  useEffect(() => { void refresh() }, [refresh])

  const projectCounts = useMemo(() => {
    const counts: Record<string, number> = { all: 0 }
    for (const it of items ?? []) {
      counts.all++
      const p = it.metadata?.projectName ?? '(no metadata)'
      counts[p] = (counts[p] ?? 0) + 1
    }
    return counts
  }, [items])

  const visible = useMemo(() => {
    if (!items) return []
    return items.filter((it) => {
      const p = it.metadata?.projectName ?? '(no metadata)'
      if (activeProject !== 'all' && p !== activeProject) return false
      if (filter.trim() === '') return true
      const q = filter.trim().toLowerCase()
      const { name, description } = resolveIdentity(it)
      return (
        it.taskId.toLowerCase().includes(q) ||
        (it.metadata?.prompt ?? '').toLowerCase().includes(q) ||
        (it.metadata?.agentPeerId ?? '').toLowerCase().includes(q) ||
        name.toLowerCase().includes(q) ||
        (description ?? '').toLowerCase().includes(q)
      )
    })
  }, [items, activeProject, filter, resolveIdentity])

  return (
    <div className="p-7 max-w-6xl">
      <SectionLabel>ASSETS</SectionLabel>
      <HeroTitle accent="artifacts">Your</HeroTitle>
      <p className="text-[15px] text-text-1 mt-2 mb-5">
        <span className="text-accent font-mono font-semibold">{items?.length ?? 0}</span>{' '}
        artifacts across <span className="text-accent font-mono font-semibold">
          {Object.keys(projectCounts).filter(k => k !== 'all').length}
        </span> projects.
      </p>

      {/* Toolbar */}
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        {Object.entries(projectCounts).map(([proj, count]) => (
          <button
            key={proj}
            onClick={() => setActiveProject(proj)}
            className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-[12px] font-medium font-mono
              transition-colors
              ${activeProject === proj
                ? 'bg-accent/[.12] border border-accent/[.5] text-accent-high'
                : 'bg-bg-1/60 border border-accent/[.15] text-text-1 hover:border-accent/[.4]'}`}
          >
            <span>{proj === 'all' ? 'All' : proj}</span>
            <span className="text-text-3 text-[11px]">{count}</span>
          </button>
        ))}
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="filter…"
          className="ml-1 bg-bg-1/60 border border-accent/[.15] focus:border-accent/[.5]
            focus:shadow-[0_0_0_3px_rgba(34,211,238,.12)] outline-none
            px-3 py-1.5 rounded-lg text-[12px] font-mono text-text-1 min-w-[220px]"
        />
        <button
          onClick={refresh}
          className="ml-auto inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-mono font-semibold
            text-text-0 border border-accent/30 hover:border-accent/55 transition-colors"
        ><RefreshCw size={12} /><span>Refresh</span></button>
      </div>

      {error && (
        <div className="bg-bad/10 border border-bad/30 rounded-lg p-4 mb-4">
          <div className="text-bad font-semibold mb-1">Couldn't list artifacts</div>
          <div className="text-text-1 text-[13px]">{error}</div>
        </div>
      )}

      {/* Table */}
      <div className="relative overflow-hidden bg-bg-1/55 border border-accent/[.15] rounded-xl">
        <span className="absolute inset-x-0 top-0 h-px overflow-hidden">
          <span className="absolute top-0 left-0 w-2/5 h-px animate-top-sweep"
                style={{background:'linear-gradient(90deg,transparent,#22d3ee,transparent)',
                        boxShadow:'0 0 8px #22d3ee'}}/>
        </span>
        {/* Header */}
        <div className="grid items-center gap-3.5 px-3.5 py-2.5 border-b border-accent/[.12]"
             style={{gridTemplateColumns:'28px 1.6fr 1fr 2fr 1.2fr 70px 80px 110px',
                     fontSize:10,letterSpacing:'0.14em'}}>
          <div></div>
          <div className="font-mono uppercase font-bold text-text-2">Agent</div>
          <div className="font-mono uppercase font-bold text-text-2">Project</div>
          <div className="font-mono uppercase font-bold text-text-2">Prompt</div>
          <div className="font-mono uppercase font-bold text-text-2">Task ID</div>
          <div className="font-mono uppercase font-bold text-text-2 text-right">Size</div>
          <div className="font-mono uppercase font-bold text-text-2 text-right">Age</div>
          <div></div>
        </div>

        {/* Rows */}
        {visible.length === 0 ? (
          <div className="text-center text-text-2 py-10 text-[14px]">
            {items === null ? 'Loading…' : 'No artifacts match.'}
          </div>
        ) : (
          visible.map((it) => {
            const project = it.metadata?.projectName ?? '(no metadata)'
            const { name, description, peerLabel } = resolveIdentity(it)
            const hasName = name.length > 0
            return (
              <div key={it.taskId}
                   className="relative group grid items-center gap-3.5 px-3.5 py-2.5
                     border-b border-accent/[.08] last:border-b-0 hover:bg-accent/[.04] transition-colors text-[13px]"
                   style={{gridTemplateColumns:'28px 1.6fr 1fr 2fr 1.2fr 70px 80px 110px'}}>
                <span className="absolute left-0 top-0 bottom-0 w-[2px] opacity-0 group-hover:opacity-100 transition-opacity"
                      style={{background:'linear-gradient(180deg,#22d3ee,#a855f7)',
                              boxShadow:'0 0 8px rgba(34,211,238,.5)'}}/>
                <div className="text-[18px] text-accent">📦</div>
                <div className="min-w-0">
                  <div className="font-semibold text-text-0 truncate text-[14px]">
                    {hasName ? name : <span className="text-text-2">Unknown agent</span>}
                  </div>
                  {peerLabel && (
                    <div className="font-mono text-[11px] text-text-2 truncate" title={it.metadata?.agentPeerId}>
                      {peerLabel}
                    </div>
                  )}
                  {description && (
                    <div className="text-[11px] text-text-2 truncate mt-0.5" title={description}>
                      {description}
                    </div>
                  )}
                </div>
                <div>
                  <span className="inline-flex font-mono text-[11px] px-2 py-0.5 rounded-full"
                        style={{
                          background: project === '(no metadata)' ? 'rgba(122,133,149,.1)'
                                    : 'rgba(168,85,247,.1)',
                          color:      project === '(no metadata)' ? '#7a8595' : '#d8b4fe',
                          border:     project === '(no metadata)' ? '1px solid rgba(122,133,149,.2)'
                                                                  : '1px solid rgba(168,85,247,.25)',
                        }}>
                    {project}
                  </span>
                </div>
                <div className={`truncate text-text-1 ${it.metadata?.prompt ? '' : 'italic text-text-3'}`}>
                  {it.metadata?.prompt ?? '(pre-metadata zip — prompt unknown)'}
                </div>
                <div className="font-mono text-accent-high truncate text-[12px]" title={it.taskId}>{it.taskId}</div>
                <div className="font-mono text-text-2 text-[12px] text-right">{formatBytes(it.sizeBytes)}</div>
                <div className="font-mono text-text-2 text-[12px] text-right">{formatAge(it.mtime)}</div>
                <div className="flex justify-end gap-1">
                  <GradientButton
                    onClick={() => {
                      void window.api.app.openArtifact(it.taskId).catch(() => {
                        toast.error('Could not open in Finder')
                      })
                    }}
                    className="px-3 py-1.5 text-[12px]"
                  >
                    <ExternalLink size={11} />
                    <span>Open</span>
                  </GradientButton>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

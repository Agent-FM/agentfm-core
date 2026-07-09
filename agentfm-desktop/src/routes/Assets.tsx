import { useEffect, useState, useCallback, useMemo } from 'react'
import { motion } from 'framer-motion'
import { toast } from 'sonner'
import { ExternalLink, RefreshCw, Package, X } from 'lucide-react'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { Button } from '../components/primitives/Button'
import { SkeletonRow } from '../components/primitives/Skeleton'
import { staggerItem } from '../lib/motion'
import { usePeerIdentityCache } from '../lib/peerIdentityCache'
import { shortenPeerID } from '../lib/peer'
import type { ArtifactListEntry } from '../../shared/ipc'

const GRID_COLS = '28px 2fr 1fr 2fr 1fr 70px 80px 92px'

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
  const [hiddenChips, setHiddenChips] = useState<Set<string>>(new Set())
  const [filter, setFilter] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const peerCache = usePeerIdentityCache((s) => s.byPeerId)

  const resolveIdentity = useCallback(
    (it: ArtifactListEntry): { name: string; description?: string; peerLabel?: string; peerId?: string } => {
      const meta = it.metadata
      const cached = meta?.agentPeerId ? peerCache[meta.agentPeerId] : undefined
      const name =
        (meta?.agentName?.trim() || cached?.name?.trim() || cached?.agent_capability?.trim()) ?? ''
      const description = meta?.agentDescription?.trim() || cached?.description?.trim()
      const peerLabel = meta?.agentPeerId ? shortenPeerID(meta.agentPeerId, 6, 5) : undefined
      return { name, description, peerLabel, peerId: meta?.agentPeerId }
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

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const copyPeer = (peerId?: string) => {
    if (!peerId) return
    navigator.clipboard
      .writeText(peerId)
      .then(() => toast.success('Peer ID copied'))
      .catch(() => toast.error('Copy failed'))
  }

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
    <div className="p-4">
      <HeroTitle accent="artifacts">Your</HeroTitle>
      <p className="text-sm text-text-1 mt-1 mb-4">
        <span className="text-accent font-mono text-xs tabular-nums">{items?.length ?? 0}</span>{' '}
        artifacts across <span className="text-accent font-mono text-xs tabular-nums">
          {Object.keys(projectCounts).filter(k => k !== 'all').length}
        </span> projects
      </p>

      {/* Toolbar */}
      <div className="flex items-center gap-1.5 mb-3 flex-wrap">
        {Object.entries(projectCounts)
          .filter(([proj]) => proj === 'all' || !hiddenChips.has(proj))
          .map(([proj, count]) => (
            <span
              key={proj}
              className={`group/chip inline-flex items-center h-[24px] rounded-ctl text-xs font-medium font-mono
                transition-colors duration-150 overflow-hidden
                ${activeProject === proj
                  ? 'bg-control-selected text-text-0'
                  : 'bg-control text-text-1 hover:bg-control-hover'}`}
            >
              <button
                onClick={() => setActiveProject(proj)}
                className={`inline-flex items-center gap-1.5 h-full px-2.5 cursor-pointer ${proj === 'all' ? 'pr-2.5' : 'pr-1'}`}
              >
                <span>{proj === 'all' ? 'All' : proj}</span>
                <span className="text-text-2 text-2xs tabular-nums">{count}</span>
              </button>
              {proj !== 'all' && (
                <button
                  onClick={() => {
                    setHiddenChips((prev) => new Set(prev).add(proj))
                    if (activeProject === proj) setActiveProject('all')
                  }}
                  aria-label={`Remove ${proj} filter`}
                  title={`Remove ${proj} filter`}
                  className="h-full px-1.5 text-text-2 hover:text-bad opacity-0 group-hover/chip:opacity-100 transition-opacity cursor-pointer"
                >
                  <X size={11} strokeWidth={2} />
                </button>
              )}
            </span>
          ))}
        {hiddenChips.size > 0 && (
          <button
            onClick={() => setHiddenChips(new Set())}
            className="text-2xs text-text-2 hover:text-text-0 px-1.5 transition-colors cursor-pointer"
            title="Restore removed filter tabs"
          >
            restore {hiddenChips.size} hidden
          </button>
        )}
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter artifacts"
          placeholder="Filter artifacts"
          className="ml-1 glass-inset focus:border-accent/50
            outline-none px-2.5 h-[22px] rounded-ctl text-xs font-mono text-text-1 min-w-[220px]"
        />
        {(filter !== '' || activeProject !== 'all') && (
          <button
            onClick={() => { setFilter(''); setActiveProject('all') }}
            aria-label="Clear filters"
            title="Clear filters"
            className="inline-flex items-center gap-1 h-[22px] px-2 rounded-ctl text-xs text-text-1 hover:text-text-0 hover:bg-white/[0.05] transition-colors duration-150 cursor-pointer"
          >
            <X size={12} strokeWidth={1.5} />
            <span>Clear filters</span>
          </button>
        )}
        <Button variant="secondary" onClick={refresh} className="ml-auto text-xs">
          <RefreshCw size={12} /><span>Refresh</span>
        </Button>
      </div>

      {error && (
        <div className="bg-bad/10 border border-bad/30 rounded-card p-3 mb-4">
          <div className="text-bad font-semibold text-sm mb-0.5">Couldn't list artifacts</div>
          <div className="text-text-1 text-sm">{error}</div>
        </div>
      )}

      {/* Table */}
      <div className="border-t border-border-0 -mx-4">
        {/* Header */}
        <div className="grid items-center gap-3 h-6 px-4 bg-chrome border-b border-border-0 text-text-2 text-2xs font-medium"
             style={{gridTemplateColumns: GRID_COLS}}>
          <div></div>
          <div>Agent</div>
          <div>Project</div>
          <div>Prompt</div>
          <div>Task ID</div>
          <div className="text-right">Size</div>
          <div className="text-right">Age</div>
          <div></div>
        </div>

        {/* Rows */}
        {items === null ? (
          <div>
            {Array.from({ length: 8 }).map((_, i) => (
              <SkeletonRow key={i} delay={i * 60} className="px-4" />
            ))}
          </div>
        ) : visible.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-14 border-b border-border-0">
            <Package size={20} strokeWidth={1.5} className="text-text-3" />
            <div className="text-sm font-medium text-text-1">No artifacts yet</div>
            <div className="text-xs text-text-3">Adjust the project or filter above.</div>
          </div>
        ) : (
          visible.map((it, i) => {
            const project = it.metadata?.projectName ?? '(no metadata)'
            const { name, description, peerLabel, peerId } = resolveIdentity(it)
            const hasName = name.length > 0
            const prompt = it.metadata?.prompt
            const longPrompt = (prompt?.length ?? 0) > 90
            const isOpen = expanded.has(it.taskId)
            return (
              <motion.div key={it.taskId} {...staggerItem(Math.min(i, 12))}>
                <div className="grid items-center gap-3 h-6 px-4 border-b border-border-0 hover:bg-white/[0.04] transition-colors duration-150 text-sm"
                     style={{gridTemplateColumns: GRID_COLS}}>
                  <div className="text-text-2"><Package size={16} strokeWidth={1.5} /></div>
                  <div className="min-w-0 flex items-center gap-1.5" title={description || undefined}>
                    <span className="truncate">
                      {hasName ? <span className="text-text-0">{name}</span> : <span className="text-text-2">Unknown agent</span>}
                    </span>
                    {peerLabel && (
                      <button
                        onClick={() => copyPeer(peerId)}
                        title={`Copy peer ID ${peerId}`}
                        className="shrink-0 font-mono text-2xs text-text-2 hover:text-text-0 transition-colors cursor-pointer tabular-nums"
                      >
                        {peerLabel}
                      </button>
                    )}
                  </div>
                  <div className="min-w-0">
                    <span className={`inline-flex max-w-full truncate font-mono text-2xs px-1.5 py-px rounded-full border ${
                      project === '(no metadata)'
                        ? 'bg-white/[0.05] border-border-0 text-text-2'
                        : 'bg-accent/10 border-accent/25 text-accent'}`}>
                      {project}
                    </span>
                  </div>
                  <div className="min-w-0 flex items-center gap-1.5">
                    <span className={`truncate ${prompt ? 'text-text-1' : 'italic text-text-2'}`}>
                      {prompt ?? 'No prompt recorded'}
                    </span>
                    {longPrompt && (
                      <button
                        onClick={() => toggle(it.taskId)}
                        className="shrink-0 text-2xs font-medium text-accent hover:text-accent-light transition-colors cursor-pointer"
                      >
                        {isOpen ? 'less' : 'more'}
                      </button>
                    )}
                  </div>
                  <div className="font-mono text-text-1 truncate text-xs tabular-nums" title={it.taskId}>{it.taskId}</div>
                  <div className="font-mono text-text-2 text-xs text-right tabular-nums">{formatBytes(it.sizeBytes)}</div>
                  <div className="font-mono text-text-2 text-xs text-right tabular-nums">{formatAge(it.mtime)}</div>
                  <div className="flex justify-end">
                    <Button
                      variant="ghost"
                      onClick={() => {
                        void window.api.app.openArtifact(it.taskId).catch(() => {
                          toast.error('Could not open in Finder')
                        })
                      }}
                      className="h-[18px] px-1.5 text-xs text-accent hover:text-accent"
                    >
                      <ExternalLink size={11} />
                      <span>Open</span>
                    </Button>
                  </div>
                </div>
                {isOpen && prompt && (
                  <div className="px-4 pl-[52px] py-2.5 border-b border-border-0 bg-white/[0.015]">
                    <div className="text-2xs font-medium text-text-2 mb-1">Full prompt</div>
                    <p className="text-sm text-text-1 leading-relaxed whitespace-pre-wrap max-w-3xl" style={{ overflowWrap: 'anywhere' }}>
                      {prompt}
                    </p>
                  </div>
                )}
              </motion.div>
            )
          })
        )}
      </div>
    </div>
  )
}

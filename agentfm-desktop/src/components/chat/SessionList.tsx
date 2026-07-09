import { Trash2, Plus } from 'lucide-react'
import type { ChatSession } from '../../types/chat'
import { compactAge } from '../../lib/peer'
import { Button } from '../primitives/Button'

interface Props {
  sessions: ChatSession[]
  activeId: string | null
  onSelect: (id: string) => void
  onNew: () => void
  onDelete?: (id: string) => void
}

export function SessionList({ sessions, activeId, onSelect, onNew, onDelete }: Props) {
  const buckets: Record<'Today' | 'Yesterday' | 'Older', ChatSession[]> = {
    Today: [], Yesterday: [], Older: [],
  }
  const now = Date.now()
  const dayMs = 24 * 60 * 60 * 1000
  for (const s of sessions) {
    const age = now - s.updatedAt
    if (age < dayMs) buckets.Today.push(s)
    else if (age < 2 * dayMs) buckets.Yesterday.push(s)
    else buckets.Older.push(s)
  }
  for (const b of Object.keys(buckets) as (keyof typeof buckets)[]) {
    buckets[b].sort((a, b2) => b2.updatedAt - a.updatedAt)
  }

  return (
    <aside className="pane w-60 bg-navigator border-r border-border-0 p-2 flex flex-col overflow-auto">
      <Button variant="primary" onClick={onNew} className="w-full">
        <Plus size={14} strokeWidth={1.5} />
        <span>New chat</span>
      </Button>

      {(['Today', 'Yesterday', 'Older'] as const).map((bucket) => {
        if (buckets[bucket].length === 0) return null
        return (
          <div key={bucket} className="mt-4">
            <div className="uppercase tracking-[0.06em] text-2xs font-medium text-text-1 px-1 mb-1">
              {bucket}
            </div>
            <div>
              {buckets[bucket].map((s) => {
                const isActive = s.id === activeId
                return (
                  <div key={s.id} className="relative group">
                    <button
                      onClick={() => onSelect(s.id)}
                      className={`w-full text-left ${onDelete ? 'pl-2 pr-8' : 'px-2'} h-6 rounded-ctl text-sm transition-colors duration-150 flex items-center gap-2 min-w-0
                        ${isActive ? 'row-selected text-text-0' : 'text-text-1 hover:bg-white/[0.04]'}`}
                    >
                      <span className="truncate flex-1 min-w-0">{s.title || 'Untitled'}</span>
                      <span className="text-2xs text-text-2 font-mono tabular-nums shrink-0 group-hover:opacity-0 transition-opacity duration-150">
                        {compactAge(s.updatedAt)}
                      </span>
                    </button>
                    {onDelete && (
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); onDelete(s.id) }}
                        aria-label={`Delete session ${s.title || 'Untitled'}`}
                        title="Delete session"
                        className="absolute right-1 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-text-2 hover:text-bad p-1 rounded-ctl transition-colors duration-150"
                      >
                        <Trash2 size={12} strokeWidth={1.5} />
                      </button>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}

      {sessions.length === 0 && (
        <div className="mt-6 text-center text-text-3 text-sm px-2">No conversations yet.</div>
      )}
    </aside>
  )
}

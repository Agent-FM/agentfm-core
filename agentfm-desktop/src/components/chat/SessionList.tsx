import { Trash2, Plus } from 'lucide-react'
import type { ChatSession } from '../../types/chat'
import { compactAge } from '../../lib/peer'

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
    <aside className="w-60 bg-bg-1/60 border-r border-accent/[.12] p-3 flex flex-col overflow-auto" style={{backdropFilter:'blur(8px)'}}>
      <button
        onClick={onNew}
        className="w-full inline-flex items-center justify-center gap-1.5 py-3 rounded-xl
          font-bold text-[14px] text-bg-0
          bg-gradient-to-br from-accent to-accent2
          shadow-[0_6px_20px_-6px_rgba(34,211,238,.6)]
          hover:shadow-[0_6px_24px_-2px_rgba(34,211,238,.75)]
          hover:brightness-110 transition-all"
      >
        <Plus size={14} strokeWidth={2.5} />
        <span>New chat</span>
      </button>

      {(['Today', 'Yesterday', 'Older'] as const).map((bucket) => {
        if (buckets[bucket].length === 0) return null
        return (
          <div key={bucket} className="mt-5">
            <div
              className="font-mono uppercase font-bold text-text-3 px-1 mb-2"
              style={{ fontSize: 10, letterSpacing: '0.16em' }}
            >
              ▌ {bucket}
            </div>
            <div className="space-y-0.5">
              {buckets[bucket].map((s) => {
                const isActive = s.id === activeId
                return (
                  <div key={s.id} className="relative group">
                    {isActive && (
                      <span
                        className="absolute left-0 top-1 bottom-1 w-[3px] rounded-r-full"
                        style={{
                          background: 'linear-gradient(180deg, #22d3ee, #a855f7)',
                          boxShadow: '0 0 8px rgba(34,211,238,.6)',
                        }}
                      />
                    )}
                    <button
                      onClick={() => onSelect(s.id)}
                      className={`w-full text-left pl-3 pr-2 py-2 rounded-md text-[14px] transition-all flex items-center gap-2
                        ${isActive ? 'bg-accent/[.1] text-text-0' : 'text-text-1 hover:bg-bg-2 hover:translate-x-px'}`}
                    >
                      <div className="flex-1 min-w-0">
                        <div className="truncate font-medium">{s.title || 'Untitled'}</div>
                        <div className="text-[10px] text-text-3 font-mono mt-0.5">{compactAge(s.updatedAt)} ago</div>
                      </div>
                      {onDelete && (
                        <span
                          role="button"
                          onClick={(e) => { e.stopPropagation(); onDelete(s.id) }}
                          className="opacity-0 group-hover:opacity-100 text-text-3 hover:text-rose-400 p-1 rounded transition-all"
                          title="Delete session"
                        >
                          <Trash2 size={12} />
                        </span>
                      )}
                    </button>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}

      {sessions.length === 0 && (
        <div className="mt-6 text-center text-text-3 text-[12px] px-2">No conversations yet.</div>
      )}
    </aside>
  )
}

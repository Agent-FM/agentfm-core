import { Button } from '../primitives/Button';
import type { ChatSession } from '../../types/chat';

interface Props {
  sessions: ChatSession[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete?: (id: string) => void;
}

export function SessionList({ sessions, activeId, onSelect, onNew, onDelete }: Props) {
  // Group by date bucket
  const buckets: Record<string, ChatSession[]> = { Today: [], Yesterday: [], Older: [] };
  const now = Date.now();
  const dayMs = 24 * 60 * 60 * 1000;
  for (const s of sessions) {
    const age = now - s.updatedAt;
    if (age < dayMs) buckets.Today.push(s);
    else if (age < 2 * dayMs) buckets.Yesterday.push(s);
    else buckets.Older.push(s);
  }

  return (
    <aside className="w-60 bg-bg-1 border-r border-border-0 p-3 flex flex-col overflow-auto">
      <Button variant="primary" className="w-full" onClick={onNew}>
        + New chat
      </Button>

      {(['Today', 'Yesterday', 'Older'] as const).map((bucket) => {
        if (buckets[bucket].length === 0) return null;
        return (
          <div key={bucket} className="mt-4">
            <div className="text-[10px] uppercase tracking-wider text-text-2 mb-1.5">
              {bucket}
            </div>
            {buckets[bucket].map((s) => {
              const isActive = s.id === activeId;
              return (
                <div key={s.id} className="group flex items-center">
                  <button
                    onClick={() => onSelect(s.id)}
                    className={`flex-1 text-left px-2.5 py-1.5 rounded-md text-sm transition-colors ${
                      isActive ? 'bg-accent-bg text-accent' : 'text-text-1 hover:bg-bg-2'
                    }`}
                  >
                    <div className="truncate">{s.title || 'Untitled'}</div>
                  </button>
                  {onDelete && (
                    <button
                      onClick={() => onDelete(s.id)}
                      className="opacity-0 group-hover:opacity-100 text-text-2 hover:text-rose-400 px-1 py-1 text-xs transition-opacity"
                      title="Delete session"
                    >
                      ×
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        );
      })}
    </aside>
  );
}

import type { EndpointDef, EndpointGroup } from '../../lib/apiCatalog'

interface Props {
  endpoints: EndpointDef[]
  selectedId: string
  onSelect: (id: string) => void
}

const GROUP_ORDER: EndpointGroup[] = ['OpenAI-compatible', 'AgentFM-native', 'System']

export function EndpointList({ endpoints, selectedId, onSelect }: Props) {
  return (
    <nav className="w-64 shrink-0 h-full overflow-auto border-r border-border-0 bg-bg-1/40 p-3">
      <button
        onClick={() => onSelect('overview')}
        aria-current={selectedId === 'overview' ? 'page' : undefined}
        className={`w-full text-left px-2.5 py-2 mb-3 rounded-lg text-[13px] font-medium transition-colors ${
          selectedId === 'overview'
            ? 'bg-accent/15 text-accent'
            : 'text-text-1 hover:bg-white/5 hover:text-text-0'
        }`}
      >
        Overview
      </button>
      {GROUP_ORDER.map((group) => {
        const items = endpoints.filter((e) => e.group === group)
        if (!items.length) return null
        return (
          <div key={group} className="mb-3">
            <div className="px-2 py-1 text-2xs uppercase tracking-wide text-text-2">{group}</div>
            {items.map((ep) => {
              const active = ep.id === selectedId
              return (
                <button
                  key={ep.id}
                  onClick={() => onSelect(ep.id)}
                  aria-current={active ? 'page' : undefined}
                  className={`w-full text-left px-2 py-1.5 rounded-lg font-mono text-[11px] flex items-center gap-1.5 transition-colors ${
                    active ? 'bg-accent/15 text-text-0' : 'text-text-1 hover:bg-white/5'
                  }`}
                >
                  <span
                    className={`shrink-0 w-9 text-[9px] font-bold tracking-wide ${
                      ep.method === 'GET' ? 'text-emerald-400' : 'text-accent'
                    }`}
                  >
                    {ep.method}
                  </span>
                  <span className="truncate">{ep.path}</span>
                </button>
              )
            })}
          </div>
        )
      })}
    </nav>
  )
}

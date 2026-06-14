import type { EndpointDef, EndpointGroup } from '../../lib/apiCatalog'

interface Props {
  endpoints: EndpointDef[]
  selectedId: string
  onSelect: (id: string) => void
}

const GROUP_ORDER: EndpointGroup[] = ['OpenAI-compatible', 'AgentFM-native', 'System']

export function EndpointList({ endpoints, selectedId, onSelect }: Props) {
  return (
    <nav className="bg-bg-2 border border-border-0 rounded-2xl p-2 w-64 shrink-0 overflow-auto">
      {GROUP_ORDER.map((group) => {
        const items = endpoints.filter((e) => e.group === group)
        if (!items.length) return null
        return (
          <div key={group} className="mb-3">
            <div className="px-2 py-1 text-2xs uppercase tracking-wide text-text-2">{group}</div>
            {items.map((ep) => (
              <button
                key={ep.id}
                onClick={() => onSelect(ep.id)}
                aria-current={ep.id === selectedId ? 'page' : undefined}
                className={`w-full text-left px-2 py-1.5 rounded-lg font-mono text-xs transition-colors ${
                  ep.id === selectedId ? 'bg-accent/15 text-text-0' : 'text-text-1 hover:bg-white/5'
                }`}
              >
                <span className="text-accent mr-1">{ep.method}</span>
                {ep.path}
              </button>
            ))}
          </div>
        )
      })}
    </nav>
  )
}

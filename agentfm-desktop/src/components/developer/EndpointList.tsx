import type { EndpointDef, EndpointGroup } from '../../lib/apiCatalog'

interface Props {
  endpoints: EndpointDef[]
  selectedId: string
  onSelect: (id: string) => void
}

const GROUP_ORDER: EndpointGroup[] = ['OpenAI-compatible', 'AgentFM-native', 'System']

export function EndpointList({ endpoints, selectedId, onSelect }: Props) {
  return (
    <nav className="pane w-64 shrink-0 h-full overflow-auto bg-navigator border-r border-border-0 py-2">
      <button
        onClick={() => onSelect('overview')}
        aria-current={selectedId === 'overview' ? 'page' : undefined}
        className={`w-full text-left h-6 px-3 mb-2 text-sm flex items-center transition-colors duration-150 ${
          selectedId === 'overview'
            ? 'row-selected text-text-0'
            : 'text-text-1 hover:bg-white/[0.04] hover:text-text-0'
        }`}
      >
        Overview
      </button>
      {GROUP_ORDER.map((group) => {
        const items = endpoints.filter((e) => e.group === group)
        if (!items.length) return null
        return (
          <div key={group} className="mb-2">
            <div className="px-3 h-6 flex items-center text-2xs font-medium text-text-2">{group}</div>
            {items.map((ep) => {
              const active = ep.id === selectedId
              return (
                <button
                  key={ep.id}
                  onClick={() => onSelect(ep.id)}
                  aria-current={active ? 'page' : undefined}
                  className={`w-full text-left h-6 px-3 font-mono text-2xs flex items-center gap-1.5 transition-colors duration-150 ${
                    active ? 'row-selected text-text-0' : 'text-text-1 hover:bg-white/[0.04]'
                  }`}
                >
                  <span
                    className={`shrink-0 w-9 text-2xs font-semibold tracking-wide ${
                      ep.method === 'GET' ? 'text-ok' : 'text-accent'
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

import { API_CATALOG } from '../lib/apiCatalog'
import { RoutePage } from '../components/primitives/RoutePage'
import { SectionLabel } from '../components/primitives/SectionLabel'

export default function Developer() {
  return (
    <RoutePage className="p-6">
      <SectionLabel>DEVELOPER</SectionLabel>
      <h1 className="text-3xl font-semibold mb-4">Developer API</h1>
      <ul className="space-y-1">
        {API_CATALOG.map((ep) => (
          <li key={ep.id} className="font-mono text-sm text-text-1">
            <span className="text-accent">{ep.method}</span> {ep.path}
          </li>
        ))}
      </ul>
    </RoutePage>
  )
}

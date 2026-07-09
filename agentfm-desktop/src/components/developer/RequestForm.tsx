import type { EndpointDef, FormValues } from '../../lib/apiCatalog'

interface Props {
  endpoint: EndpointDef
  values: FormValues
  onChange: (next: FormValues) => void
}

export function RequestForm({ endpoint, values, onChange }: Props) {
  const set = (name: string, value: string) => onChange({ ...values, [name]: value })
  const bodyParam = endpoint.params.find((p) => p.loc === 'body')
  const scalarParams = endpoint.params.filter((p) => p.loc !== 'body')

  return (
    <div className="space-y-3">
      {scalarParams.map((p) => (
        <label key={p.name} className="block">
          <span className="text-xs text-text-2">
            {p.name}
            {p.required && <span className="text-bad"> *</span>}
            <span className="ml-2 text-text-2">{p.loc}</span>
          </span>
          <input
            aria-label={p.name}
            value={values[p.name] ?? ''}
            placeholder={String(p.example ?? '')}
            onChange={(e) => set(p.name, e.target.value)}
            className="mt-1 w-full glass-inset rounded-ctl px-3 py-2 text-sm font-mono text-text-0 outline-none focus:border-accent/40"
          />
        </label>
      ))}

      {bodyParam && (
        <label className="block">
          <span className="text-xs text-text-2">Request body (JSON)</span>
          <textarea
            aria-label="Request body (JSON)"
            value={values[bodyParam.name] ?? JSON.stringify(bodyParam.example, null, 2)}
            onChange={(e) => set(bodyParam.name, e.target.value)}
            spellCheck={false}
            className="mt-1 w-full h-44 glass-inset rounded-ctl px-3 py-2 text-xs font-mono text-text-0 outline-none focus:border-accent/40"
          />
        </label>
      )}
    </div>
  )
}

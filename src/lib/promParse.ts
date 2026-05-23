import type { MetricSample, MetricType } from '../types/metrics'

const TYPE_RE = /^#\s*TYPE\s+(\S+)\s+(counter|gauge|histogram|summary)\s*$/
const SAMPLE_RE = /^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+(-?[\d.eE+-]+|NaN|\+Inf|-Inf)\s*$/
const LABEL_RE = /([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"/g

function parseLabels(raw: string | undefined): Record<string, string> {
  if (!raw) return {}
  const out: Record<string, string> = {}
  LABEL_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = LABEL_RE.exec(raw)) !== null) {
    out[m[1]] = m[2].replace(/\\(.)/g, '$1')
  }
  return out
}

function parseValue(raw: string): number {
  if (raw === '+Inf') return Infinity
  if (raw === '-Inf') return -Infinity
  if (raw === 'NaN') return NaN
  return Number(raw)
}

function baseName(name: string): string {
  if (name.endsWith('_bucket')) return name.slice(0, -'_bucket'.length)
  if (name.endsWith('_sum')) return name.slice(0, -'_sum'.length)
  if (name.endsWith('_count')) return name.slice(0, -'_count'.length)
  return name
}

export function parseMetrics(text: string): MetricSample[] {
  const out: MetricSample[] = []
  const types = new Map<string, MetricType>()
  const lines = text.split('\n')
  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) continue
    try {
      if (line.startsWith('#')) {
        const t = line.match(TYPE_RE)
        if (t) types.set(t[1], t[2] as MetricType)
        continue
      }
      const m = line.match(SAMPLE_RE)
      if (!m) continue
      const name = m[1]
      const labels = parseLabels(m[2])
      const value = parseValue(m[3])
      const type = types.get(baseName(name)) ?? 'unknown'
      out.push({ name, labels, value, type })
    } catch {
      // Defensive: malformed lines must not abort the rest of the parse.
    }
  }
  return out
}

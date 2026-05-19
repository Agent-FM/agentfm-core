interface Option<T> {
  value: T
  label: string
}

interface Props<T extends string> {
  options: Option<T>[]
  value: T
  onChange: (v: T) => void
}

export function SegGroup<T extends string>({ options, value, onChange }: Props<T>) {
  return (
    <div className="inline-flex p-1 bg-bg-2 border border-border-0 rounded-md gap-1">
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          className={`relative px-3 py-1 text-xs rounded transition-colors ${
            o.value === value
              ? 'text-accent gradient-border-cyan bg-accent-bg'
              : 'text-text-2 hover:text-text-0'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

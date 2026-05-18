interface SegGroupProps<T extends string> {
  options: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
}

export function SegGroup<T extends string>({ options, value, onChange }: SegGroupProps<T>) {
  return (
    <div className="inline-flex bg-bg-0 border border-border-0 rounded-md p-0.5">
      {options.map((opt) => (
        <button
          key={opt.value}
          onClick={() => onChange(opt.value)}
          className={`px-3 py-1.5 text-xs rounded transition-colors ${
            value === opt.value ? 'bg-accent-bg text-accent font-medium' : 'text-text-2 hover:text-text-0'
          }`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}

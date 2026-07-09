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
    <div className="bg-raised inline-flex p-px rounded-ctl gap-px">
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          aria-pressed={o.value === value}
          className={`px-2.5 h-5 text-xs rounded-[4px] transition-colors duration-150 cursor-pointer ${
            o.value === value
              ? 'bg-control-selected text-text-0 font-medium'
              : 'text-text-1 hover:text-text-0'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

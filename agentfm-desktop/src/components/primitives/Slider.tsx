interface Props {
  min: number
  max: number
  step?: number
  value: number
  onChange: (v: number) => void
  label?: string
}

export function Slider({ min, max, step = 0.01, value, onChange, label }: Props) {
  const pct = ((value - min) / (max - min)) * 100
  return (
    <div className="relative w-full h-3 flex items-center">
      <div className="w-full h-[3px] rounded-full bg-white/[0.08] overflow-hidden">
        <div
          className="h-full bg-accent"
          style={{ width: `${pct}%` }}
        />
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        aria-label={label}
        onChange={(e) => onChange(Number(e.target.value))}
        className="absolute inset-0 w-full opacity-0 cursor-pointer"
      />
      <div
        className="absolute top-1/2 -translate-y-1/2 w-2.5 h-2.5 rounded-full bg-text-0 border border-border-1 pointer-events-none"
        style={{ left: `calc(${pct}% - 5px)` }}
      />
    </div>
  )
}

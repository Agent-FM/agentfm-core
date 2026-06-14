interface Props {
  min: number
  max: number
  step?: number
  value: number
  onChange: (v: number) => void
}

export function Slider({ min, max, step = 0.01, value, onChange }: Props) {
  const pct = ((value - min) / (max - min)) * 100
  return (
    <div className="relative w-full">
      <div className="h-1.5 rounded-full bg-bg-2 overflow-hidden">
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
        onChange={(e) => onChange(Number(e.target.value))}
        className="absolute inset-0 w-full opacity-0 cursor-pointer"
      />
      <div
        className="absolute top-1/2 -translate-y-1/2 w-3 h-3 rounded-full bg-accent border border-accent-fg shadow-[0_0_8px_#22d3ee] pointer-events-none"
        style={{ left: `calc(${pct}% - 6px)` }}
      />
    </div>
  )
}

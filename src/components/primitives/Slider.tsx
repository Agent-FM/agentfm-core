interface SliderProps {
  min: number
  max: number
  step?: number
  value: number
  onChange: (v: number) => void
}

export function Slider({ min, max, step = 0.01, value, onChange }: SliderProps) {
  const pct = ((value - min) / (max - min)) * 100
  return (
    <div className="relative h-4 flex items-center">
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="absolute inset-0 w-full opacity-0 cursor-pointer"
      />
      <div className="absolute inset-x-0 h-1 bg-bg-2 rounded-full" />
      <div className="absolute h-1 bg-accent rounded-full" style={{ width: `${pct}%` }} />
      <div
        className="absolute w-4 h-4 rounded-full bg-accent shadow-[0_0_0_4px_color-mix(in_srgb,var(--accent)_15%,transparent)] -translate-x-1/2"
        style={{ left: `${pct}%` }}
      />
    </div>
  )
}

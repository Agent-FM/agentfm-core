interface Props {
  value: number
}

export function Meter({ value }: Props) {
  const clamped = Math.max(0, Math.min(100, value))
  return (
    <div
      className="relative overflow-hidden rounded-full bg-bg-1 border border-border-0"
      style={{ height: 5, marginTop: 7 }}
    >
      <span
        data-meter-fill
        className="block h-full rounded-full bg-accent transition-[width] duration-300 ease-out"
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}

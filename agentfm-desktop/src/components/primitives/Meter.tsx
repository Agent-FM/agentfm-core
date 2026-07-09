interface Props {
  value: number
}

export function Meter({ value }: Props) {
  const clamped = Math.max(0, Math.min(100, value))
  const fill =
    clamped > 90 ? 'bg-bad' : clamped > 70 ? 'bg-warn' : 'bg-accent'
  return (
    <div
      className="relative overflow-hidden rounded-full bg-white/[0.08]"
      style={{ height: 3, marginTop: 6 }}
    >
      <span
        data-meter-fill
        className={`block h-full rounded-full transition-[width] duration-200 ease-out ${fill}`}
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}

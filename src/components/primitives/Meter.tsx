interface Props {
  value: number
}

export function Meter({ value }: Props) {
  const clamped = Math.max(0, Math.min(100, value))
  return (
    <div
      className="relative overflow-hidden rounded-full bg-bg-0/85 border border-accent/10"
      style={{ height: 5, marginTop: 7 }}
    >
      <span
        data-meter-fill
        className="block h-full rounded-full"
        style={{
          width: `${clamped}%`,
          background: 'linear-gradient(90deg, #22d3ee, #a855f7)',
        }}
      />
      <span
        className="absolute inset-0 w-1/2 animate-meter-shimmer"
        style={{
          background:
            'linear-gradient(90deg, transparent, rgba(255,255,255,.4), transparent)',
        }}
      />
    </div>
  )
}

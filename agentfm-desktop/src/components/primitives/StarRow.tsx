import { Star, StarHalf } from 'lucide-react'

interface Props {
  value: number
  size?: number
}

export function StarRow({ value, size = 13 }: Props) {
  return (
    <span className="inline-flex items-center gap-0.5 align-middle" aria-label={`${value} of 5 stars`}>
      {Array.from({ length: 5 }, (_, i) => {
        const filled = value >= i + 1
        const half = !filled && value >= i + 0.5
        if (half) return <StarHalf key={i} size={size} className="fill-accent text-accent" />
        return (
          <Star
            key={i}
            size={size}
            className={filled ? 'fill-accent text-accent' : 'text-text-3'}
          />
        )
      })}
    </span>
  )
}

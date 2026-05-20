import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  accent: string
  trail?: ReactNode
}

export function HeroTitle({ children, accent, trail }: Props) {
  return (
    <h1
      className="font-bold tracking-tight leading-[1.05]"
      style={{ fontSize: '44px', letterSpacing: '-0.025em', margin: 0 }}
    >
      {children}{' '}
      <span
        className="hero-shimmer animate-text-shimmer"
        style={{
          background: 'linear-gradient(120deg, #22d3ee, #a855f7, #22d3ee)',
          backgroundSize: '200% 100%',
          WebkitBackgroundClip: 'text',
          backgroundClip: 'text',
          color: 'transparent',
        }}
      >
        {accent}
      </span>
      {trail ? <> {trail}</> : null}
    </h1>
  )
}

import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  accent: string
  trail?: ReactNode
}

export function HeroTitle({ children, accent, trail }: Props) {
  return (
    <h1 className="font-semibold tracking-tight leading-tight m-0" style={{ fontSize: '33px', letterSpacing: '-0.02em' }}>
      {children}{' '}
      <span data-hero-accent className="text-accent">{accent}</span>
      {trail ? <> {trail}</> : null}
    </h1>
  )
}

import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  accent: string
  trail?: ReactNode
}

export function HeroTitle({ children, accent, trail }: Props) {
  return (
    <h1 className="text-lg font-semibold m-0 text-text-0">
      {children}{' '}
      <span data-hero-accent className="text-accent">{accent}</span>
      {trail != null ? <> {trail}</> : null}
    </h1>
  )
}

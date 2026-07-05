export type DotTone = 'cyan' | 'violet' | 'amber' | 'rose' | 'lime' | 'neutral'

const COLOR: Record<DotTone, string> = {
  cyan:    'bg-accent shadow-[0_0_8px_rgba(247,147,30,.7)]',
  violet:  'bg-accent shadow-[0_0_8px_rgba(247,147,30,.7)]',
  amber:   'bg-warn shadow-[0_0_8px_rgba(245,158,11,.7)]',
  rose:    'bg-bad shadow-[0_0_8px_rgba(244,63,94,.7)]',
  lime:    'bg-ok shadow-[0_0_8px_rgba(132,204,22,.7)]',
  neutral: 'bg-text-3',
}

interface Props {
  tone?: DotTone
  pulse?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export function StatusDot({ tone = 'cyan', pulse = false, size = 'md', className }: Props) {
  const dim = size === 'sm' ? 'w-1.5 h-1.5' : size === 'lg' ? 'w-3 h-3' : 'w-2 h-2'
  const animation = pulse ? 'animate-pulse-cyan' : ''
  return (
    <span className={`inline-block rounded-full ${dim} ${COLOR[tone]} ${animation} ${className ?? ''}`} />
  )
}

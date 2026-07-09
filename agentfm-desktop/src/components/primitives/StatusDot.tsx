export type DotTone = 'accent' | 'ok' | 'warn' | 'bad' | 'neutral'

const COLOR: Record<DotTone, string> = {
  accent:  'bg-accent',
  ok:      'bg-ok',
  warn:    'bg-warn',
  bad:     'bg-bad',
  neutral: 'bg-text-3',
}

interface Props {
  tone?: DotTone
  pulse?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export function StatusDot({ tone = 'accent', pulse = false, size = 'md', className }: Props) {
  const dim = size === 'sm' ? 'w-1.5 h-1.5' : size === 'lg' ? 'w-3 h-3' : 'w-2 h-2'
  const animation = pulse ? 'animate-pulse-cyan' : ''
  return (
    <span className={`inline-block rounded-full ${dim} ${COLOR[tone]} ${animation} ${className ?? ''}`} />
  )
}

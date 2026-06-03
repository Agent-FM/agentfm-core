import { ChevronDown } from 'lucide-react'

interface Props {
  name: string
  onClick?: () => void
}

export function ProjectChip({ name, onClick }: Props) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-2 bg-accent/[.08] border border-accent/30
        px-3.5 py-2 rounded-full text-[13px] font-medium hover:border-accent/50 transition-colors"
    >
      <span
        className="w-1.5 h-1.5 rounded-full bg-accent animate-pulse-cyan"
        style={{ boxShadow: '0 0 8px #22d3ee' }}
      />
      <span className="text-text-0">{name}</span>
      <ChevronDown size={12} className="text-text-2" />
    </button>
  )
}

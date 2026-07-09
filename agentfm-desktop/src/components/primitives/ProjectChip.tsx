import { ChevronDown } from 'lucide-react'

interface Props {
  name: string
  onClick?: () => void
}

export function ProjectChip({ name, onClick }: Props) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-1.5 h-[22px] bg-raised border border-border-1
        px-2 rounded-ctl font-mono text-2xs font-medium hover:bg-control transition-colors"
    >
      <span className="w-1 h-1 rounded-full bg-accent" />
      <span className="text-text-0">{name}</span>
      <ChevronDown size={11} strokeWidth={1.5} className="text-text-2" />
    </button>
  )
}

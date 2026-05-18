export type ProjectColor = 'emerald' | 'violet' | 'rose' | 'cyan' | 'amber'

export interface Project {
  id: string
  name: string
  icon: string
  color: ProjectColor
  relayMultiaddr: string | null
  reputationFloor: number
  createdAt: number
}

export const PROJECT_COLORS: ProjectColor[] = [
  'emerald',
  'violet',
  'rose',
  'cyan',
  'amber',
]

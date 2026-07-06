export type ConnectionMode = 'public' | 'private'

export interface Project {
  id: string
  name: string
  relayMultiaddr: string | null
  reputationFloor: number
  createdAt: number
  connectionMode: ConnectionMode
  swarmKey: string | null
}

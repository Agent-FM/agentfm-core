export interface Project {
  id: string
  name: string
  relayMultiaddr: string | null
  reputationFloor: number
  createdAt: number
}

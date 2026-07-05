import Store from 'electron-store'

export interface StoredProject {
  id: string
  reputationFloor: number
  relayMultiaddr: string | null
  connectionMode: 'public' | 'private'
  swarmKey: string | null
}

interface SettingsShape {
  theme: 'dark' | 'light' | 'auto'
  accent: 'emerald' | 'violet' | 'rose'
  apiPort: number
  reputationFloor: number
  relayMultiaddr: string | null
  telemetry: boolean
  projects: StoredProject[]
  activeProjectId: string | null
}

const defaults: SettingsShape = {
  theme: 'dark',
  accent: 'emerald',
  apiPort: 8080,
  reputationFloor: -0.5,
  relayMultiaddr: null,
  telemetry: false,
  projects: [],
  activeProjectId: null,
}

export const settingsStore = new Store<SettingsShape>({
  defaults,
  name: 'settings',
})

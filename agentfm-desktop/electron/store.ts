import Store from 'electron-store'

interface SettingsShape {
  theme: 'dark' | 'light' | 'auto'
  accent: 'emerald' | 'violet' | 'rose'
  apiPort: number
  reputationFloor: number
  relayMultiaddr: string | null
  telemetry: boolean
}

const defaults: SettingsShape = {
  theme: 'dark',
  accent: 'emerald',
  apiPort: 8080,
  reputationFloor: -0.5,
  relayMultiaddr: null,
  telemetry: false,
}

export const settingsStore = new Store<SettingsShape>({
  defaults,
  name: 'settings',
})

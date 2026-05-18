import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { queryClient } from './lib/query'
import { loadApiPortFromSettings, setApiPort } from './lib/api'
import { useUIStore } from './lib/store'
import App from './App'
import './styles/globals.css'

async function bootstrap() {
  await loadApiPortFromSettings()

  // Hydrate all settings from electron-store
  try {
    const theme = await window.api?.settings.get<'dark' | 'light' | 'auto'>('theme')
    const accent = await window.api?.settings.get<'emerald' | 'violet' | 'rose'>('accent')
    const apiPort = await window.api?.settings.get<number>('apiPort')
    const reputationFloor = await window.api?.settings.get<number>('reputationFloor')
    const relayMultiaddr = await window.api?.settings.get<string | null>('relayMultiaddr')

    const store = useUIStore.getState()
    if (theme) store.setTheme(theme)
    if (accent) store.setAccent(accent)
    if (typeof apiPort === 'number') {
      store.setApiPort(apiPort)
      setApiPort(apiPort)
    }
    if (typeof reputationFloor === 'number') store.setReputationFloor(reputationFloor)
    if (relayMultiaddr !== undefined) store.setRelayMultiaddr(relayMultiaddr)
  } catch {}

  // Apply stored or default values to <html> attributes
  const { theme, accent } = useUIStore.getState()
  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.setAttribute('data-accent', accent)

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <App />
        <Toaster theme="dark" position="bottom-right" />
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

bootstrap()

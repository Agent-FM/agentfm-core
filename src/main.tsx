import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { queryClient } from './lib/query'
import { loadApiPortFromSettings, setApiPort } from './lib/api'
import { useUIStore } from './lib/store'
import { migrateLegacySettings } from './lib/projectMigration'
import { normalizeProject } from './lib/projectStore'
import type { Project } from './types/project'
import App from './App'
import './styles/globals.css'

async function bootstrap() {
  await loadApiPortFromSettings()

  await migrateLegacySettings({
    get: (k) => window.api.settings.get(k),
    set: (k, v) => window.api.settings.set(k, v),
    delete: (k) => window.api.settings.delete(k),
  } as never).catch((e) => console.warn('migration failed', e))

  const rawProjects = (await window.api.settings.get<Project[]>('projects')) ?? []
  const projects = rawProjects.map(normalizeProject)
  const activeId = (await window.api.settings.get<string | null>('activeProjectId')) ?? null
  useUIStore.getState().hydrateProjects(projects, activeId)

  try {
    const theme = await window.api?.settings.get<'dark' | 'light' | 'auto'>('theme')
    const accent = await window.api?.settings.get<'emerald' | 'violet' | 'rose'>('accent')
    const apiPort = await window.api?.settings.get<number>('apiPort')

    const store = useUIStore.getState()
    if (theme) store.setTheme(theme)
    if (accent) store.setAccent(accent)
    if (typeof apiPort === 'number') {
      store.setApiPort(apiPort)
      setApiPort(apiPort)
    }
  } catch {}

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

import type { Project } from '../types/project'
import type { ChatSession } from '../types/chat'

export const DEFAULT_PROJECT_ID = 'prj_default'

interface MigrationStore {
  get<T>(key: string): Promise<T | undefined>
  set(key: string, value: unknown): Promise<void>
  delete(key: string): Promise<void>
}

export async function migrateLegacySettings(store: MigrationStore): Promise<void> {
  const existing = await store.get<Project[]>('projects')
  if (Array.isArray(existing) && existing.length > 0) return

  const legacyRelay = (await store.get<string | null>('relayMultiaddr')) ?? null
  const legacyFloor = (await store.get<number>('reputationFloor')) ?? -0.5
  const legacySessions = (await store.get<ChatSession[]>('chat:sessions')) ?? []

  const defaultProject: Project = {
    id: DEFAULT_PROJECT_ID,
    name: 'Default',
    icon: '🌐',
    color: 'emerald',
    relayMultiaddr: legacyRelay,
    reputationFloor: legacyFloor,
    createdAt: Date.now(),
  }

  await store.set('projects', [defaultProject])
  await store.set('activeProjectId', DEFAULT_PROJECT_ID)
  if (legacySessions.length > 0) {
    await store.set(`chat:sessions:${DEFAULT_PROJECT_ID}`, legacySessions)
  }

  await store.delete('chat:sessions').catch(() => {})
  await store.delete('relayMultiaddr').catch(() => {})
  await store.delete('reputationFloor').catch(() => {})
}

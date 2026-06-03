import { describe, it, expect } from 'vitest'
import { migrateLegacySettings, DEFAULT_PROJECT_ID } from '../../src/lib/projectMigration'

interface Store {
  get<T>(key: string): Promise<T | undefined>
  set(key: string, value: unknown): Promise<void>
  delete(key: string): Promise<void>
}

function fakeStore(initial: Record<string, unknown>): Store {
  const data: Record<string, unknown> = { ...initial }
  return {
    get: async <T,>(k: string) => data[k] as T | undefined,
    set: async (k: string, v: unknown) => {
      data[k] = v
    },
    delete: async (k: string) => {
      delete data[k]
    },
  }
}

describe('migrateLegacySettings', () => {
  it('does nothing when projects already exist', async () => {
    const store = fakeStore({
      projects: [{ id: 'prj_keep', name: 'X', relayMultiaddr: null, reputationFloor: -0.5, createdAt: 1 }],
    })
    await migrateLegacySettings(store)
    expect(await store.get('projects')).toHaveLength(1)
  })

  it('creates a Default project from legacy relay + floor', async () => {
    const store = fakeStore({
      relayMultiaddr: '/ip4/127.0.0.1/tcp/4001/p2p/12D3LegacyTest',
      reputationFloor: -0.3,
      'chat:sessions': [{ id: 'chat_x', title: 'legacy', pinnedPeerId: null, preferredModel: 'auto', messages: [], createdAt: 1, updatedAt: 1 }],
    })
    await migrateLegacySettings(store)
    const projects = (await store.get('projects')) as unknown[]
    expect(projects).toHaveLength(1)
    expect((projects[0] as { id: string }).id).toBe(DEFAULT_PROJECT_ID)
    expect((projects[0] as { relayMultiaddr: string }).relayMultiaddr).toBe(
      '/ip4/127.0.0.1/tcp/4001/p2p/12D3LegacyTest',
    )
    expect((projects[0] as { reputationFloor: number }).reputationFloor).toBe(-0.3)
    expect(await store.get('activeProjectId')).toBe(DEFAULT_PROJECT_ID)
    expect(await store.get(`chat:sessions:${DEFAULT_PROJECT_ID}`)).toHaveLength(1)
    expect(await store.get('chat:sessions')).toBeUndefined()
    expect(await store.get('relayMultiaddr')).toBeUndefined()
    expect(await store.get('reputationFloor')).toBeUndefined()
  })

  it('uses defaults when legacy fields are missing', async () => {
    const store = fakeStore({})
    await migrateLegacySettings(store)
    const projects = (await store.get('projects')) as unknown[]
    expect(projects).toHaveLength(1)
    expect((projects[0] as { relayMultiaddr: string | null }).relayMultiaddr).toBeNull()
    expect((projects[0] as { reputationFloor: number }).reputationFloor).toBe(-0.5)
  })

  it('is idempotent', async () => {
    const store = fakeStore({})
    await migrateLegacySettings(store)
    const after1 = await store.get('projects')
    await migrateLegacySettings(store)
    const after2 = await store.get('projects')
    expect(after2).toEqual(after1)
  })
})

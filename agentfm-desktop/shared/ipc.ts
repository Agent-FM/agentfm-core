// Shared IPC types used by BOTH the Electron preload (node project) and the
// renderer (web project). Kept in shared/ — included by tsconfig.web.json and
// tsconfig.node.json — so the renderer doesn't import across the electron/
// project boundary (which triggers TS6307).

export interface ArtifactMetadata {
  projectName?: string
  prompt?: string
  agentName?: string
  agentDescription?: string
  agentPeerId?: string
}

export interface ArtifactListEntry {
  taskId: string
  sizeBytes: number
  mtime: number
  metadata?: ArtifactMetadata
}

export interface SaveSwarmKeyResult {
  ok: boolean
  path?: string
  error?: string
}

export interface ApiBridge {
  platform: string
  backend: {
    health: () => Promise<unknown>
    restart: (cfg?: unknown) => Promise<unknown>
    logs: (n?: number) => Promise<string[]>
    onCrash: (cb: (data: unknown) => void) => () => void
    onFailed: (cb: (data: unknown) => void) => () => void
  }
  settings: {
    get: <T = unknown>(key: string) => Promise<T | undefined>
    set: (key: string, value: unknown) => Promise<unknown>
    delete: (key: string) => Promise<void>
  }
  app: {
    openExternal: (url: string) => Promise<unknown>
    showItemInFolder: (path: string) => Promise<unknown>
    checkArtifact: (taskId: string) => Promise<boolean>
    openArtifact: (taskId: string) => Promise<unknown>
    listArtifacts: () => Promise<ArtifactListEntry[]>
    writeArtifactMeta: (taskId: string, meta: ArtifactMetadata) => Promise<void>
    saveSwarmKeyFile: (content: string, defaultName: string) => Promise<SaveSwarmKeyResult>
  }
}

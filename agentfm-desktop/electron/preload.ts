import { contextBridge, ipcRenderer } from 'electron'
import type { ApiBridge, ArtifactMetadata, ArtifactListEntry } from '../shared/ipc'

export type { ApiBridge, ArtifactMetadata, ArtifactListEntry } from '../shared/ipc'

const api: ApiBridge = {
  platform: process.platform,
  backend: {
    health: () => ipcRenderer.invoke('backend:health'),
    restart: (cfg?: unknown) => ipcRenderer.invoke('backend:restart', cfg),
    logs: (n?: number) => ipcRenderer.invoke('backend:logs', n),
    onCrash: (cb: (data: unknown) => void) => {
      const listener = (_: Electron.IpcRendererEvent, data: unknown) => cb(data)
      ipcRenderer.on('backend:crashed', listener)
      return () => ipcRenderer.removeListener('backend:crashed', listener)
    },
    onFailed: (cb: (data: unknown) => void) => {
      const listener = (_: Electron.IpcRendererEvent, data: unknown) => cb(data)
      ipcRenderer.on('backend:failed', listener)
      return () => ipcRenderer.removeListener('backend:failed', listener)
    },
  },
  settings: {
    get: <T = unknown>(key: string) =>
      ipcRenderer.invoke('settings:get', key) as Promise<T | undefined>,
    set: (key: string, value: unknown) => ipcRenderer.invoke('settings:set', key, value),
    delete: (key: string) => ipcRenderer.invoke('settings:delete', key) as Promise<void>,
  },
  app: {
    openExternal: (url: string) => ipcRenderer.invoke('app:openExternal', url),
    showItemInFolder: (path: string) => ipcRenderer.invoke('app:showItemInFolder', path),
    checkArtifact: (taskId: string) => ipcRenderer.invoke('app:checkArtifact', taskId) as Promise<boolean>,
    openArtifact: (taskId: string) => ipcRenderer.invoke('app:openArtifact', taskId),
    listArtifacts: () => ipcRenderer.invoke('app:listArtifacts') as Promise<ArtifactListEntry[]>,
    writeArtifactMeta: (taskId: string, meta: ArtifactMetadata) =>
      ipcRenderer.invoke('app:writeArtifactMeta', taskId, meta) as Promise<void>,
    saveSwarmKeyFile: (content: string, defaultName: string) =>
      ipcRenderer.invoke('app:saveSwarmKeyFile', content, defaultName) as Promise<{
        ok: boolean
        path?: string
        error?: string
      }>,
  },
}

contextBridge.exposeInMainWorld('api', api)

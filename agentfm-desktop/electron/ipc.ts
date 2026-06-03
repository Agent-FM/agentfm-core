import { ipcMain, shell, BrowserWindow } from 'electron'
import { BackendManager } from './backend-manager'
import { settingsStore } from './store'

export function registerIPC(backend: BackendManager): void {
  // Backend controls
  ipcMain.handle('backend:health', () => backend.health())
  ipcMain.handle('backend:restart', (_event, cfg) => backend.restart(cfg))
  ipcMain.handle('backend:logs', (_event, n?: number) => backend.logs(n))

  // Persistent settings
  ipcMain.handle('settings:get', (_event, key: string) => settingsStore.get(key as never))
  ipcMain.handle('settings:set', (_event, key: string, value: unknown) =>
    settingsStore.set(key as never, value as never),
  )
  ipcMain.handle('settings:delete', (_event, key: string) => {
    settingsStore.delete(key as never)
  })

  // Shell helpers
  ipcMain.handle('app:openExternal', (_event, url: string) => shell.openExternal(url))
  ipcMain.handle('app:showItemInFolder', (_event, path: string) => shell.showItemInFolder(path))

  // Artifact helpers
  ipcMain.handle('app:checkArtifact', (_event, taskId: string) => backend.artifactExists(taskId))
  ipcMain.handle('app:openArtifact', (_event, taskId: string) => {
    const artifactPath = backend.getArtifactPath(taskId)
    if (backend.artifactExists(taskId)) shell.showItemInFolder(artifactPath)
  })
  ipcMain.handle('app:listArtifacts', () => backend.listArtifacts())
  ipcMain.handle('app:writeArtifactMeta', (_event, taskId: string, meta: unknown) => {
    if (typeof taskId !== 'string' || !meta || typeof meta !== 'object') return
    backend.writeArtifactMeta(taskId, meta as Parameters<typeof backend.writeArtifactMeta>[1])
  })

  // Forward backend events to all renderer windows
  backend.on('crashed', (data) => {
    BrowserWindow.getAllWindows().forEach((w) => w.webContents.send('backend:crashed', data))
  })
  backend.on('failed', (data) => {
    BrowserWindow.getAllWindows().forEach((w) => w.webContents.send('backend:failed', data))
  })
}

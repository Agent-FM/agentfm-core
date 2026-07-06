import { ipcMain, shell, BrowserWindow } from 'electron'
import { resolve, sep } from 'node:path'
import { BackendManager } from './backend-manager'
import { settingsStore } from './store'
import { sanitizePort, isValidMultiaddr } from './validate'

const SAFE_EXTERNAL_PROTOCOLS = new Set(['http:', 'https:', 'mailto:'])

export function isSafeExternalUrl(url: string): boolean {
  try {
    return SAFE_EXTERNAL_PROTOCOLS.has(new URL(url).protocol)
  } catch {
    return false
  }
}

export function registerIPC(backend: BackendManager): void {
  // Backend controls
  ipcMain.handle('backend:health', () => backend.health())
  ipcMain.handle('backend:restart', (_event, cfg) => {
    if (cfg && typeof cfg === 'object') {
      const c = cfg as { apiPort?: unknown; relayMultiaddr?: unknown }
      if (c.apiPort !== undefined) {
        ;(cfg as { apiPort: number }).apiPort = sanitizePort(c.apiPort)
      }
      if (c.relayMultiaddr != null && c.relayMultiaddr !== '') {
        if (!isValidMultiaddr(c.relayMultiaddr)) {
          throw new Error(`invalid relayMultiaddr: ${JSON.stringify(c.relayMultiaddr)}`)
        }
      }
    }
    return backend.restart(cfg)
  })
  ipcMain.handle('backend:logs', (_event, n?: number) => backend.logs(n))

  // Persistent settings
  ipcMain.handle('settings:get', (_event, key: string) => settingsStore.get(key as never))
  ipcMain.handle('settings:set', (_event, key: string, value: unknown) => {
    let safeValue = value
    if (key === 'apiPort') {
      safeValue = sanitizePort(value)
    } else if (key === 'relayMultiaddr') {
      if (value != null && value !== '') {
        if (!isValidMultiaddr(value)) {
          throw new Error(`invalid relayMultiaddr: ${JSON.stringify(value)}`)
        }
      }
    }
    settingsStore.set(key as never, safeValue as never)
  })
  ipcMain.handle('settings:delete', (_event, key: string) => {
    settingsStore.delete(key as never)
  })

  // Shell helpers. Both are renderer-reachable, so treat their arguments as
  // untrusted: openExternal is limited to web/mail schemes (file:, smb: etc.
  // would open local content), showItemInFolder to the workspace dir.
  ipcMain.handle('app:openExternal', (_event, url: string) => {
    if (!isSafeExternalUrl(url)) return
    return shell.openExternal(url)
  })
  ipcMain.handle('app:showItemInFolder', (_event, path: string) => {
    if (typeof path !== 'string') return
    const resolved = resolve(path)
    if (!resolved.startsWith(backend.artifactsDir + sep)) return
    shell.showItemInFolder(resolved)
  })

  // Artifact helpers
  ipcMain.handle('app:checkArtifact', (_event, taskId: string) => backend.artifactExists(taskId))
  ipcMain.handle('app:openArtifact', (_event, taskId: string) => {
    if (!backend.artifactExists(taskId)) return
    shell.showItemInFolder(backend.getArtifactPath(taskId))
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

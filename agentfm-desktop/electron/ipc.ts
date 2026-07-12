import { ipcMain, shell, BrowserWindow, dialog } from 'electron'
import { resolve, sep } from 'node:path'
import { writeFile } from 'node:fs/promises'
import { BackendManager } from './backend-manager'
import { settingsStore } from './store'
import { sanitizePort, isValidMultiaddr } from './validate'

const SAFE_EXTERNAL_PROTOCOLS = new Set(['http:', 'https:', 'mailto:'])

// Top-level settings keys the renderer is allowed to write. Anything else
// (including electron-store dot-notation like "apiPort.x") is refused so a
// compromised renderer can't smuggle unvalidated values into spawn args.
const ALLOWED_SETTINGS_KEYS = new Set([
  'theme',
  'accent',
  'apiPort',
  'reputationFloor',
  'relayMultiaddr',
  'telemetry',
  'projects',
  'activeProjectId',
])

// Chat sessions persist under a per-project key (chat:sessions:<projectId>),
// so they can't live in the static allowlist. Permit exactly that shape with
// a conservative project-id suffix (matches the ids minted in projectStore).
const CHAT_SESSIONS_KEY = /^chat:sessions:[A-Za-z0-9_-]{1,64}$/

// isAllowedSettingsKey gates every renderer settings:set. A key passes when it
// is a dot-free member of the static allowlist OR a well-formed per-project
// chat-sessions key. Exported for unit testing.
export function isAllowedSettingsKey(key: unknown): key is string {
  if (typeof key !== 'string' || key.includes('.')) return false
  return ALLOWED_SETTINGS_KEYS.has(key) || CHAT_SESSIONS_KEY.test(key)
}

const SWARM_KEY_HEX = /^[0-9a-fA-F]{64}$/

// validateProjects checks the security-relevant fields of each stored project
// before they can reach the backend spawn args (relayMultiaddr → -bootstrap,
// swarmKey → PSK) or a per-project path (id). Throws on anything invalid.
function validateProjects(value: unknown): unknown {
  if (!Array.isArray(value)) {
    throw new Error('projects must be an array')
  }
  for (const entry of value) {
    if (!entry || typeof entry !== 'object') {
      throw new Error('invalid project entry')
    }
    const p = entry as Record<string, unknown>
    if (typeof p.id !== 'string' || p.id.length === 0 || p.id.length > 128 || p.id.includes('/') || p.id.includes('..')) {
      throw new Error(`invalid project id: ${JSON.stringify(p.id)}`)
    }
    if (p.relayMultiaddr != null && p.relayMultiaddr !== '' && !isValidMultiaddr(p.relayMultiaddr)) {
      throw new Error(`invalid project relayMultiaddr: ${JSON.stringify(p.relayMultiaddr)}`)
    }
    if (p.swarmKey != null && p.swarmKey !== '' && (typeof p.swarmKey !== 'string' || !SWARM_KEY_HEX.test(p.swarmKey))) {
      throw new Error('invalid project swarmKey (want 64 hex chars)')
    }
  }
  return value
}

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
    if (!isAllowedSettingsKey(key)) {
      throw new Error(`refused settings key: ${JSON.stringify(key)}`)
    }
    let safeValue = value
    if (key === 'apiPort') {
      safeValue = sanitizePort(value)
    } else if (key === 'relayMultiaddr') {
      if (value != null && value !== '') {
        if (!isValidMultiaddr(value)) {
          throw new Error(`invalid relayMultiaddr: ${JSON.stringify(value)}`)
        }
      }
    } else if (key === 'projects') {
      safeValue = validateProjects(value)
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

  // Export the private swarm key to a user-chosen file, written 0600.
  ipcMain.handle('app:saveSwarmKeyFile', async (_event, content: unknown, defaultName: unknown) => {
    if (typeof content !== 'string' || !SWARM_KEY_HEX.test(content)) {
      return { ok: false, error: 'invalid swarm key' }
    }
    const suggested =
      typeof defaultName === 'string' && defaultName ? defaultName.replace(/[^\w.-]/g, '_') : 'swarm.key'
    const win = BrowserWindow.getFocusedWindow() ?? BrowserWindow.getAllWindows()[0]
    if (!win) return { ok: false, error: 'no window' }
    const res = await dialog.showSaveDialog(win, {
      title: 'Export swarm key',
      defaultPath: suggested,
      filters: [{ name: 'Swarm key', extensions: ['key'] }],
    })
    if (res.canceled || !res.filePath) return { ok: false }
    try {
      await writeFile(res.filePath, content, { mode: 0o600 })
      return { ok: true, path: res.filePath }
    } catch (err) {
      return { ok: false, error: String(err) }
    }
  })

  // Forward backend events to all renderer windows
  backend.on('crashed', (data) => {
    BrowserWindow.getAllWindows().forEach((w) => w.webContents.send('backend:crashed', data))
  })
  backend.on('failed', (data) => {
    BrowserWindow.getAllWindows().forEach((w) => w.webContents.send('backend:failed', data))
  })
}

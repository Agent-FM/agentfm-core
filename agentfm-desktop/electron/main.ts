import { app, BrowserWindow, shell, nativeImage } from 'electron'
import { join } from 'path'
import { existsSync } from 'fs'
import { is } from '@electron-toolkit/utils'
import { BackendManager } from './backend-manager'
import { registerIPC, isSafeExternalUrl } from './ipc'
import { settingsStore } from './store'

let backend: BackendManager | null = null

function resolveAppIcon(): Electron.NativeImage | undefined {
  const candidates = [
    join(__dirname, '../../build/icon.png'),
    join(process.resourcesPath, 'build', 'icon.png'),
  ]
  for (const path of candidates) {
    if (existsSync(path)) {
      const img = nativeImage.createFromPath(path)
      if (!img.isEmpty()) return img
    }
  }
  return undefined
}

function createWindow(): void {
  const appIcon = resolveAppIcon()
  const mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    show: false,
    icon: appIcon,
    autoHideMenuBar: true,
    titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default',
    backgroundColor: '#1F1F24',
    vibrancy: process.platform === 'darwin' ? 'under-window' : undefined,
    visualEffectState: 'active',
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      sandbox: true,
      contextIsolation: true,
      nodeIntegration: false,
    },
  })

  mainWindow.on('ready-to-show', () => {
    mainWindow.show()
  })

  mainWindow.webContents.setWindowOpenHandler((details) => {
    if (isSafeExternalUrl(details.url)) shell.openExternal(details.url)
    return { action: 'deny' }
  })

  if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
    mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
    mainWindow.webContents.openDevTools({ mode: 'detach' })
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

// Single-instance guard: a second launch must NOT spin up a rival backend on
// the same port (which would SIGKILL the first instance's healthy backend via
// the stale-port sweep and crash-loop both). Focus the existing window instead.
const gotSingleInstanceLock = app.requestSingleInstanceLock()
if (!gotSingleInstanceLock) {
  app.quit()
}

app.on('second-instance', () => {
  const win = BrowserWindow.getAllWindows()[0]
  if (win) {
    if (win.isMinimized()) win.restore()
    win.focus()
  }
})

app.whenReady().then(async () => {
  if (!gotSingleInstanceLock) return
  if (process.platform === 'darwin' && app.dock) {
    const dockIcon = resolveAppIcon()
    if (dockIcon) app.dock.setIcon(dockIcon)
  }

  const projects = settingsStore.get('projects') ?? []
  const activeProject = projects.find((p) => p.id === settingsStore.get('activeProjectId'))

  backend = new BackendManager({
    apiPort: settingsStore.get('apiPort'),
    reputationFloor: activeProject?.reputationFloor ?? settingsStore.get('reputationFloor'),
    relayMultiaddr: (activeProject?.relayMultiaddr ?? settingsStore.get('relayMultiaddr')) ?? undefined,
    projectId: activeProject?.id,
    swarmKey:
      activeProject?.connectionMode === 'private' ? activeProject.swarmKey ?? undefined : undefined,
  })

  registerIPC(backend)

  // Forward backend lifecycle events to console for dev visibility
  backend.on('started', ({ pid }: { pid: number }) => {
    console.log(`[backend] started (pid=${pid})`)
  })
  backend.on('crashed', (data: unknown) => {
    console.warn('[backend] crashed', data)
  })
  backend.on('failed', (data: unknown) => {
    console.error('[backend] failed permanently', data)
  })

  try {
    await backend.start()
  } catch (err) {
    console.error('Backend failed to start:', err)
  }

  createWindow()

  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

// Electron does not await async before-quit listeners, so preventDefault,
// stop the backend (SIGTERM → 5s → SIGKILL escalation lives in stop()), then
// exit for real. Guard against re-entry so the second before-quit is a no-op.
let isQuitting = false
app.on('before-quit', (event) => {
  if (isQuitting || !backend) return
  event.preventDefault()
  isQuitting = true
  backend.stop().finally(() => app.exit(0))
})

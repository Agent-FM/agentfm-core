// Renames the dev Electron.app bundle to "AgentFM" so the macOS Dock and menu
// show the product name during `npm run dev`. The Dock reads CFBundleName from
// the running .app bundle, which app.setName() cannot override in dev — only
// this can. No-op on non-macOS or when the bundle is absent (CI / fresh clone
// before electron is installed). Packaged builds get their name from
// electron-builder.yml (productName) and never hit this path.
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'

if (process.platform !== 'darwin') process.exit(0)

const plist = 'node_modules/electron/dist/Electron.app/Contents/Info.plist'
if (!existsSync(plist)) process.exit(0)

function setKey(key, value) {
  try {
    execFileSync('/usr/libexec/PlistBuddy', ['-c', `Set :${key} ${value}`, plist])
  } catch {
    try {
      execFileSync('/usr/libexec/PlistBuddy', ['-c', `Add :${key} string ${value}`, plist])
    } catch {
      // best-effort dev convenience; never fail the build over it
    }
  }
}

setKey('CFBundleName', 'AgentFM')
setKey('CFBundleDisplayName', 'AgentFM')

/** @vitest-environment node */

import { describe, it, expect, vi } from 'vitest'

vi.mock('electron', () => ({
  ipcMain: { handle: vi.fn() },
  shell: { openExternal: vi.fn(), showItemInFolder: vi.fn() },
  BrowserWindow: { getAllWindows: vi.fn(() => []) },
  app: { getPath: () => '/tmp/agentfm-test', isPackaged: false },
}))
vi.mock('../../electron/store', () => ({ settingsStore: { get: vi.fn(), set: vi.fn(), delete: vi.fn() } }))

import { isSafeExternalUrl } from '../../electron/ipc'

describe('isSafeExternalUrl', () => {
  it('allows web and mail links', () => {
    expect(isSafeExternalUrl('https://agentfm.network/docs')).toBe(true)
    expect(isSafeExternalUrl('http://127.0.0.1:8080/health')).toBe(true)
    expect(isSafeExternalUrl('mailto:conduct@agentfm.network')).toBe(true)
  })

  it('refuses local-content and garbage schemes', () => {
    expect(isSafeExternalUrl('file:///etc/passwd')).toBe(false)
    expect(isSafeExternalUrl('smb://attacker/share')).toBe(false)
    expect(isSafeExternalUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeExternalUrl('not a url')).toBe(false)
  })
})

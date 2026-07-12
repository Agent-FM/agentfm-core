/** @vitest-environment node */

import { describe, it, expect, vi } from 'vitest'

vi.mock('electron', () => ({
  ipcMain: { handle: vi.fn() },
  shell: { openExternal: vi.fn(), showItemInFolder: vi.fn() },
  BrowserWindow: { getAllWindows: vi.fn(() => []) },
  app: { getPath: () => '/tmp/agentfm-test', isPackaged: false },
}))
vi.mock('../../electron/store', () => ({ settingsStore: { get: vi.fn(), set: vi.fn(), delete: vi.fn() } }))

import { isSafeExternalUrl, isAllowedSettingsKey } from '../../electron/ipc'

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

describe('isAllowedSettingsKey', () => {
  it('allows the static settings keys', () => {
    expect(isAllowedSettingsKey('theme')).toBe(true)
    expect(isAllowedSettingsKey('projects')).toBe(true)
    expect(isAllowedSettingsKey('activeProjectId')).toBe(true)
  })

  it('allows per-project chat-session keys', () => {
    expect(isAllowedSettingsKey('chat:sessions:prj_default')).toBe(true)
    expect(isAllowedSettingsKey('chat:sessions:prj_private_ag')).toBe(true)
    expect(isAllowedSettingsKey('chat:sessions:prj_h4af90vy')).toBe(true)
  })

  it('refuses malformed or unlisted keys', () => {
    expect(isAllowedSettingsKey('apiPort.x')).toBe(false)
    expect(isAllowedSettingsKey('chat:sessions:')).toBe(false)
    expect(isAllowedSettingsKey('chat:sessions:bad id!')).toBe(false)
    expect(isAllowedSettingsKey('chat:sessions:a.b')).toBe(false)
    expect(isAllowedSettingsKey('arbitrary')).toBe(false)
    expect(isAllowedSettingsKey(42 as unknown)).toBe(false)
  })
})

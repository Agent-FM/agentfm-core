/** @vitest-environment node */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import * as fs from 'node:fs'

// ---------- mocks must be declared before any imports that use them ----------

vi.mock('electron', () => ({
  app: {
    getPath: (_: string) => '/tmp/agentfm-test',
    isPackaged: false,
  },
}))

// We need a reference to the mock spawn function that tests can configure
const mockSpawn = vi.fn()

vi.mock('node:child_process', () => ({
  spawn: mockSpawn,
}))

// Stub fs so we don't touch the real filesystem
vi.mock('node:fs', async () => {
  const actual = await vi.importActual<typeof fs>('node:fs')
  return {
    ...actual,
    existsSync: vi.fn(() => true),
    mkdirSync: vi.fn(),
    appendFileSync: vi.fn(),
    readFileSync: vi.fn(() => 'line1\nline2\nline3'),
    statSync: vi.fn(() => ({ size: 1024 })),
    renameSync: vi.fn(),
  }
})

// Stub global fetch used by health()
const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// ---------- import under test AFTER mocks are set up ----------
const { BackendManager } = await import('../../electron/backend-manager')

// ---------- helpers ----------

function makeFakeProc() {
  const proc = new EventEmitter() as EventEmitter & {
    pid: number
    stdout: EventEmitter
    stderr: EventEmitter
    kill: ReturnType<typeof vi.fn>
  }
  proc.pid = 12345
  proc.stdout = new EventEmitter()
  proc.stderr = new EventEmitter()
  proc.kill = vi.fn()
  return proc
}

function healthOk() {
  mockFetch.mockResolvedValue({
    ok: true,
    json: async () => ({ online_workers: 2 }),
  })
}

function healthFail() {
  mockFetch.mockRejectedValue(new Error('connection refused'))
}

// ---------- tests ----------

describe('BackendManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('emits "started" and resolves when backend becomes healthy', async () => {
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthOk()

    const mgr = new BackendManager({ apiPort: 8080, reputationFloor: -0.5 })
    const startedPayloads: unknown[] = []
    mgr.on('started', (d) => startedPayloads.push(d))

    await mgr.start()

    expect(startedPayloads).toHaveLength(1)
    expect((startedPayloads[0] as { pid: number }).pid).toBe(12345)
    expect(mockSpawn).toHaveBeenCalledOnce()
    const [_bin, args] = mockSpawn.mock.calls[0]
    expect(args).toContain('-mode')
    expect(args).toContain('api')
    expect(args).toContain('8080')
  })

  it('throws if backend never becomes healthy within timeout', async () => {
    vi.useFakeTimers()
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthFail()

    const mgr = new BackendManager({ apiPort: 8080, reputationFloor: -0.5 })

    // Attach a rejection handler before starting to prevent unhandled rejection
    const startPromise = mgr.start().catch((err: Error) => err)
    await vi.runAllTimersAsync()

    const result = await startPromise
    expect(result).toBeInstanceOf(Error)
    expect((result as Error).message).toMatch('did not become healthy')
    vi.useRealTimers()
  }, 15000)

  it('honors a custom healthTimeoutMs in the error message', async () => {
    vi.useFakeTimers()
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthFail()

    const mgr = new BackendManager({
      apiPort: 8080,
      reputationFloor: -0.5,
      healthTimeoutMs: 2500,
    })
    const startPromise = mgr.start().catch((err: Error) => err)
    await vi.runAllTimersAsync()

    const result = await startPromise
    expect(result).toBeInstanceOf(Error)
    expect((result as Error).message).toMatch('2500ms')
    vi.useRealTimers()
  }, 15000)

  it('emits "crashed" on unexpected exit and schedules restart', async () => {
    vi.useFakeTimers()
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthOk()

    const mgr = new BackendManager({ apiPort: 8080, reputationFloor: -0.5 })
    await mgr.start()

    const crashedPayloads: unknown[] = []
    mgr.on('crashed', (d) => crashedPayloads.push(d))

    // Simulate unexpected exit (not stopping = false)
    proc.emit('exit', 1, null)

    expect(crashedPayloads).toHaveLength(1)
    expect((crashedPayloads[0] as { code: number }).code).toBe(1)

    // After backoff, it should attempt restart (spawn called again)
    const secondProc = makeFakeProc()
    mockSpawn.mockReturnValue(secondProc)
    healthOk()
    await vi.runAllTimersAsync()

    expect(mockSpawn).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  }, 15000)

  it('emits "failed" after 3+ crashes within 60s window', async () => {
    vi.useFakeTimers()
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthOk()

    const mgr = new BackendManager({ apiPort: 8080, reputationFloor: -0.5 })
    await mgr.start()

    const failedPayloads: unknown[] = []
    mgr.on('failed', (d) => failedPayloads.push(d))

    // Simulate 4 crashes in quick succession (within 60s window)
    // Each crash triggers restart which calls spawn again, then exits again
    let currentProc: ReturnType<typeof makeFakeProc> = proc

    for (let i = 0; i < 4; i++) {
      const nextProc = makeFakeProc()
      mockSpawn.mockReturnValue(nextProc)
      healthOk()

      currentProc.emit('exit', 1, null)
      await vi.runAllTimersAsync()
      currentProc = nextProc
    }

    expect(failedPayloads.length).toBeGreaterThanOrEqual(1)
    vi.useRealTimers()
  }, 30000)

  it('does not restart when stopping flag is set (clean stop)', async () => {
    const proc = makeFakeProc()
    mockSpawn.mockReturnValue(proc)
    healthOk()

    const mgr = new BackendManager({ apiPort: 8080, reputationFloor: -0.5 })
    await mgr.start()

    const crashedPayloads: unknown[] = []
    mgr.on('crashed', (d) => crashedPayloads.push(d))

    // Trigger stop — which sets stopping=true and kills proc
    // Simulate the proc exiting after SIGTERM
    const stopPromise = mgr.stop()
    proc.emit('exit', 0, 'SIGTERM')
    await stopPromise

    // No crash event should have fired
    expect(crashedPayloads).toHaveLength(0)
    // Spawn should only have been called once (initial start)
    expect(mockSpawn).toHaveBeenCalledOnce()
  })
})

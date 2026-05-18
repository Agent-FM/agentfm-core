import { spawn, ChildProcess, execSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { app } from 'electron'
import { EventEmitter } from 'node:events'
import { Logger } from './logger'

export interface BackendConfig {
  apiPort: number
  reputationFloor: number
  relayMultiaddr?: string
  /** How long to wait for /health to return ok during start(). Default 10000ms. */
  healthTimeoutMs?: number
}

const DEFAULT_HEALTH_TIMEOUT_MS = 10_000

export class BackendManager extends EventEmitter {
  private proc: ChildProcess | null = null
  private cfg: BackendConfig
  private logger: Logger
  private crashCount = 0
  private crashWindowStart = 0
  private stopping = false
  readonly artifactsDir: string

  constructor(cfg: BackendConfig) {
    super()
    this.cfg = cfg
    this.logger = new Logger('backend')
    this.artifactsDir = join(app.getPath('userData'), 'workspace')
    mkdirSync(this.artifactsDir, { recursive: true })
  }

  getArtifactPath(taskId: string): string {
    return join(this.artifactsDir, 'agentfm_artifacts', taskId + '.zip')
  }

  artifactExists(taskId: string): boolean {
    return existsSync(this.getArtifactPath(taskId))
  }

  async start(): Promise<void> {
    if (this.proc) return
    this.stopping = false

    const bin = this.resolveBinary()
    if (!existsSync(bin)) {
      throw new Error(`agentfm binary not found at ${bin}`)
    }

    // Defensive: HMR or a previous crashed dev run can leave an agentfm
    // backend bound to apiPort. Spawning a new one would fail with
    // "address already in use". On Unix we sweep stray PIDs that match
    // both the binary path and our target port before spawning.
    this.killStaleOnPort(bin, this.cfg.apiPort)

    const args = [
      '-mode', 'api',
      '-apiport', String(this.cfg.apiPort),
      '--reputation-floor', String(this.cfg.reputationFloor),
    ]
    if (this.cfg.relayMultiaddr) args.push('-bootstrap', this.cfg.relayMultiaddr)

    this.logger.info(`Starting backend: ${bin} ${args.join(' ')}`)

    const proc = spawn(bin, args, { stdio: ['ignore', 'pipe', 'pipe'], cwd: this.artifactsDir })
    this.proc = proc

    proc.stdout?.on('data', (data: Buffer) => this.logger.write(data))
    proc.stderr?.on('data', (data: Buffer) => this.logger.write(data))
    proc.on('exit', (code, signal) => this.onExit(code, signal))

    this.emit('started', { pid: proc.pid })
    this.logger.info(`Backend spawned with PID ${proc.pid}`)

    const timeoutMs = this.cfg.healthTimeoutMs ?? DEFAULT_HEALTH_TIMEOUT_MS
    const deadline = Date.now() + timeoutMs
    while (Date.now() < deadline) {
      try {
        const ok = await this.health()
        if (ok.ok) {
          this.logger.info('Backend health check passed')
          return
        }
      } catch {
        // ignore — backend may not be ready yet
      }
      await new Promise((r) => setTimeout(r, 200))
    }
    throw new Error(`backend did not become healthy within ${timeoutMs}ms`)
  }

  async stop(): Promise<void> {
    this.stopping = true
    if (!this.proc) return
    const proc = this.proc
    this.logger.info('Stopping backend (SIGTERM)...')
    proc.kill('SIGTERM')

    await new Promise<void>((resolve) => {
      const timeout = setTimeout(() => {
        this.logger.error('Backend did not exit within 5s — sending SIGKILL')
        proc.kill('SIGKILL')
        resolve()
      }, 5000)
      proc.once('exit', () => {
        clearTimeout(timeout)
        this.logger.info('Backend exited cleanly')
        resolve()
      })
    })
    this.proc = null
  }

  async restart(cfg?: Partial<BackendConfig>): Promise<void> {
    await this.stop()
    if (cfg) this.cfg = { ...this.cfg, ...cfg }
    await this.start()
  }

  async health(): Promise<{ ok: boolean; online_workers: number }> {
    const ctrl = new AbortController()
    const t = setTimeout(() => ctrl.abort(), 1000)
    try {
      const res = await fetch(`http://127.0.0.1:${this.cfg.apiPort}/health`, {
        signal: ctrl.signal,
      })
      clearTimeout(t)
      if (!res.ok) return { ok: false, online_workers: 0 }
      const body = await res.json() as { online_workers?: number }
      return { ok: true, online_workers: body.online_workers ?? 0 }
    } catch {
      clearTimeout(t)
      return { ok: false, online_workers: 0 }
    }
  }

  logs(n: number = 200): string[] {
    if (!existsSync(this.logger.path)) return []
    const raw = readFileSync(this.logger.path, 'utf8')
    return raw.split('\n').slice(-n)
  }

  private onExit(code: number | null, signal: string | null): void {
    this.proc = null
    if (this.stopping) return

    const now = Date.now()
    if (now - this.crashWindowStart > 60000) {
      this.crashCount = 0
      this.crashWindowStart = now
    }
    this.crashCount++

    const lastLogs = this.logs(100)
    this.logger.error(`Backend exited unexpectedly (code=${code}, signal=${signal}), crash #${this.crashCount}`)

    if (this.crashCount > 3) {
      this.emit('failed', { code, signal, lastLogs })
      return
    }

    this.emit('crashed', { code, signal, lastLogs })

    const backoff = 1000 * this.crashCount
    this.logger.info(`Restarting backend in ${backoff}ms...`)
    setTimeout(() => {
      this.start().catch((err) => this.emit('failed', { error: String(err) }))
    }, backoff)
  }

  private killStaleOnPort(bin: string, port: number): void {
    if (process.platform === 'win32') return
    try {
      const out = execSync(`lsof -nP -i :${port} -sTCP:LISTEN -t || true`, {
        encoding: 'utf8',
        timeout: 1500,
      }).trim()
      if (!out) return
      const pids = out
        .split('\n')
        .map((p) => p.trim())
        .filter(Boolean)
      for (const pidStr of pids) {
        const pid = Number(pidStr)
        if (!Number.isFinite(pid)) continue
        try {
          const cmd = execSync(`ps -p ${pid} -o command=`, { encoding: 'utf8', timeout: 500 })
            .toString()
            .trim()
          if (!cmd.includes(bin) && !cmd.includes('agentfm')) continue
          this.logger.info(`Killing stale backend pid ${pid} on port ${port} (${cmd.slice(0, 80)})`)
          try {
            process.kill(pid, 'SIGKILL')
          } catch {
            // process already gone
          }
        } catch {
          // ps failed — process probably died
        }
      }
    } catch {
      // lsof unavailable — best effort
    }
  }

  private resolveBinary(): string {
    if (process.env.AGENTFM_BIN) return process.env.AGENTFM_BIN

    if (app.isPackaged) {
      const ext = process.platform === 'win32' ? '.exe' : ''
      const platform = `${process.platform}-${process.arch}`
      return join(process.resourcesPath, 'bin', `agentfm-${platform}${ext}`)
    }

    // Dev: relative from electron/ -> project root -> agentfm-core
    return join(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm')
  }
}

import { spawn, ChildProcess, execSync } from 'node:child_process'
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { join } from 'node:path'
import { app } from 'electron'
import { EventEmitter } from 'node:events'
import { Logger } from './logger'
import { sanitizePort } from './validate'

export interface BackendConfig {
  apiPort: number
  reputationFloor: number
  relayMultiaddr?: string
  /** Hex-encoded 64-char PSK. When set, boss is launched with -swarmkey pointing
   *  at a 0600 PSK file written into userData/swarms. */
  swarmKey?: string
  /** Project id, used to namespace the per-project swarm-key file path. */
  projectId?: string
  /** How long to wait for /health to return ok during start(). Default 10000ms. */
  healthTimeoutMs?: number
}

const SWARM_KEY_HEX = /^[0-9a-fA-F]{64}$/
const SAFE_PROJECT_ID = /^[A-Za-z0-9_-]{1,64}$/

export interface ArtifactMetadata {
  projectName?: string
  prompt?: string
  agentName?: string
  agentDescription?: string
  agentPeerId?: string
}

const SAFE_TASK_ID = /^[A-Za-z0-9_.-]{1,128}$/

export interface ArtifactListEntry {
  taskId: string
  sizeBytes: number
  mtime: number
  metadata?: ArtifactMetadata
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
    if (!SAFE_TASK_ID.test(taskId)) {
      throw new Error(`unsafe taskId: ${JSON.stringify(taskId)}`)
    }
    return join(this.artifactsDir, 'agentfm_artifacts', taskId + '.zip')
  }

  artifactExists(taskId: string): boolean {
    if (!SAFE_TASK_ID.test(taskId)) return false
    return existsSync(this.getArtifactPath(taskId))
  }

  /**
   * Per-project ledger SQLite path under userData. Returns null if no
   * projectId is set (boss falls back to its compiled-in
   * ~/.agentfm/api_ledger.db default). Splits storage per project so
   * switching swarms in the desktop doesn't commingle inbox entries
   * across PSK boundaries — without this, /api/workers?include_offline
   * surfaces ghosts from prior swarms because the SQLite file is
   * shared.
   */
  private projectLedgerPath(): string | null {
    const pid = this.cfg.projectId
    if (!pid) return null
    if (!SAFE_PROJECT_ID.test(pid)) {
      this.logger.error(`Refusing unsafe projectId '${pid}' for ledger path`)
      return null
    }
    const dir = join(app.getPath('userData'), 'ledgers', pid)
    mkdirSync(dir, { recursive: true, mode: 0o700 })
    return join(dir, 'api_ledger.db')
  }

  private materializeSwarmKey(): string | null {
    const hex = this.cfg.swarmKey
    if (!hex) return null
    if (!SWARM_KEY_HEX.test(hex)) {
      this.logger.error(`Refusing malformed swarm key (want 64 hex chars, got len=${hex.length})`)
      return null
    }
    const pid = this.cfg.projectId ?? 'default'
    if (!SAFE_PROJECT_ID.test(pid)) {
      this.logger.error(`Refusing unsafe projectId '${pid}' for swarm-key path`)
      return null
    }
    const dir = join(app.getPath('userData'), 'swarms')
    mkdirSync(dir, { recursive: true, mode: 0o700 })
    const path = join(dir, pid + '.key')
    const contents = `/key/swarm/psk/1.0.0/\n/base16/\n${hex.toLowerCase()}\n`
    writeFileSync(path, contents, { encoding: 'utf8', mode: 0o600 })
    this.logger.info(`Swarm key materialized at ${path}`)
    return path
  }

  writeArtifactMeta(taskId: string, meta: ArtifactMetadata): void {
    if (!SAFE_TASK_ID.test(taskId)) return
    const dir = join(this.artifactsDir, 'agentfm_artifacts')
    mkdirSync(dir, { recursive: true })
    const path = join(dir, taskId + '.meta.json')
    writeFileSync(path, JSON.stringify(meta), { encoding: 'utf8', mode: 0o600 })
  }

  listArtifacts(): ArtifactListEntry[] {
    const dir = join(this.artifactsDir, 'agentfm_artifacts')
    if (!existsSync(dir)) return []
    const out: ArtifactListEntry[] = []
    for (const name of readdirSync(dir)) {
      if (!name.endsWith('.zip')) continue
      const taskId = name.slice(0, -4)
      const full = join(dir, name)
      let sizeBytes = 0
      let mtime = Date.now()
      try {
        const st = statSync(full)
        sizeBytes = st.size
        mtime = st.mtimeMs
      } catch {
        continue
      }
      let metadata: ArtifactMetadata | undefined
      const metaPath = join(dir, taskId + '.meta.json')
      if (existsSync(metaPath)) {
        try {
          metadata = JSON.parse(readFileSync(metaPath, 'utf8')) as ArtifactMetadata
        } catch {
          // Corrupt sidecar — display without metadata.
        }
      }
      out.push({ taskId, sizeBytes, mtime, metadata })
    }
    out.sort((a, b) => b.mtime - a.mtime)
    return out
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

    const swarmKeyPath = this.materializeSwarmKey()
    if (swarmKeyPath) args.push('-swarmkey', swarmKeyPath)

    const ledgerPath = this.projectLedgerPath()
    if (ledgerPath) args.push('-ledger-path', ledgerPath)

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
    let safePort: number
    try {
      safePort = sanitizePort(port)
    } catch {
      this.logger.info(`killStaleOnPort: skipping sweep, invalid port ${JSON.stringify(port)}`)
      return
    }
    try {
      const out = execSync(`lsof -nP -i :${safePort} -sTCP:LISTEN -t || true`, {
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
          this.logger.info(`Killing stale backend pid ${pid} on port ${safePort} (${cmd.slice(0, 80)})`)
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

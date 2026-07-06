import { existsSync, mkdirSync, appendFileSync, statSync, renameSync } from 'node:fs'
import { join } from 'node:path'
import { app } from 'electron'

const MAX_SIZE_BYTES = 5 * 1024 * 1024 // 5 MB per file
const MAX_ROTATIONS = 3

export class Logger {
  private logPath: string
  private logsDir: string

  constructor(name: string) {
    this.logsDir = join(app.getPath('userData'), 'logs')
    if (!existsSync(this.logsDir)) {
      mkdirSync(this.logsDir, { recursive: true })
    }
    this.logPath = join(this.logsDir, `${name}.log`)
  }

  write(data: string | Buffer): void {
    this.rotateIfNeeded()
    appendFileSync(this.logPath, data)
  }

  info(msg: string): void {
    this.write(`[${new Date().toISOString()}] INFO  ${msg}\n`)
  }

  error(msg: string): void {
    this.write(`[${new Date().toISOString()}] ERROR ${msg}\n`)
  }

  get path(): string {
    return this.logPath
  }

  private rotateIfNeeded(): void {
    if (!existsSync(this.logPath)) return
    try {
      const { size } = statSync(this.logPath)
      if (size < MAX_SIZE_BYTES) return
      // Rotate: .log -> .1.log -> .2.log -> .3.log (drop oldest)
      for (let i = MAX_ROTATIONS - 1; i >= 1; i--) {
        const older = `${this.logPath}.${i}`
        const newer = `${this.logPath}.${i + 1}`
        if (existsSync(older)) renameSync(older, newer)
      }
      renameSync(this.logPath, `${this.logPath}.1`)
    } catch {
      // Ignore rotation errors — keep writing to current file
    }
  }
}

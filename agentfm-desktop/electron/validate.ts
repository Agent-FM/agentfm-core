const MULTIADDR_RE = /^\/[A-Za-z0-9/._\-:]+$/

export function isValidMultiaddr(value: unknown): boolean {
  if (value == null || value === '') return true
  if (typeof value !== 'string') return false
  return MULTIADDR_RE.test(value)
}

export function sanitizePort(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  if (typeof value === 'string' && value.trim() === '') {
    throw new Error('invalid port: empty string')
  }
  if (!Number.isInteger(n) || n < 1 || n > 65535) {
    throw new Error(`invalid port: ${JSON.stringify(value)}`)
  }
  return n
}

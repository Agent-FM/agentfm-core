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

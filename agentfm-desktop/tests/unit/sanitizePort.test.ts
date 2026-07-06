/** @vitest-environment node */

import { describe, it, expect } from 'vitest'
import { sanitizePort } from '../../electron/validate'

describe('sanitizePort', () => {
  it('accepts valid integer ports (number or numeric string)', () => {
    expect(sanitizePort(8080)).toBe(8080)
    expect(sanitizePort('8080')).toBe(8080)
    expect(sanitizePort(1)).toBe(1)
    expect(sanitizePort(65535)).toBe(65535)
  })

  it('rejects a shell-injection payload instead of coercing it', () => {
    expect(() => sanitizePort('0; touch /tmp/pwned #')).toThrow()
    expect(() => sanitizePort('8080 && rm -rf ~')).toThrow()
    expect(() => sanitizePort('$(whoami)')).toThrow()
  })

  it('rejects out-of-range and non-integer values', () => {
    expect(() => sanitizePort(0)).toThrow()
    expect(() => sanitizePort(-1)).toThrow()
    expect(() => sanitizePort(65536)).toThrow()
    expect(() => sanitizePort(99999)).toThrow()
    expect(() => sanitizePort(80.5)).toThrow()
    expect(() => sanitizePort('abc')).toThrow()
    expect(() => sanitizePort('')).toThrow()
    expect(() => sanitizePort(null)).toThrow()
    expect(() => sanitizePort(undefined)).toThrow()
  })
})

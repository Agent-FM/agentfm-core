import { describe, it, expect } from 'vitest'
import { stripAnsi } from '../../src/lib/ansi'

describe('stripAnsi', () => {
  it('removes color escape codes', () => {
    expect(stripAnsi('\x1b[92mhello\x1b[0m world')).toBe('hello world')
  })

  it('removes 256-color codes', () => {
    expect(stripAnsi('\x1b[38;5;202mhi\x1b[0m')).toBe('hi')
  })

  it('removes cursor-motion codes', () => {
    expect(stripAnsi('a\x1b[2Kclear\x1b[1Aup')).toBe('aclearup')
  })

  it('leaves regular text untouched', () => {
    expect(stripAnsi('plain text')).toBe('plain text')
  })

  it('preserves box-drawing characters', () => {
    const box = '╭──────╮\n│ hi   │\n╰──────╯'
    expect(stripAnsi(box)).toBe(box)
  })

  it('handles empty input', () => {
    expect(stripAnsi('')).toBe('')
  })
})

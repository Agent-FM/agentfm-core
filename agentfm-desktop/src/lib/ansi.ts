const CSI = /\x1b\[[0-?]*[ -/]*[@-~]/g
const OSC = /\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g

export function stripAnsi(s: string): string {
  if (!s) return s
  return s.replace(CSI, '').replace(OSC, '')
}

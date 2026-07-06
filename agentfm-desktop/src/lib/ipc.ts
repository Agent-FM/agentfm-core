import type { ApiBridge } from '../../electron/preload'

declare global {
  interface Window {
    api: ApiBridge
  }
}

export const ipc = window.api

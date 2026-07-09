import type { ApiBridge } from '../../shared/ipc'

declare global {
  interface Window {
    api: ApiBridge
  }
}

export const ipc = window.api

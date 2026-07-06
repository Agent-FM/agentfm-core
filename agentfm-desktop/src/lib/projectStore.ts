import type { ConnectionMode, Project } from '../types/project'

export class DuplicateRelayError extends Error {
  constructor(public relay: string | null) {
    super(
      relay === null
        ? 'another project already uses the bundled public lighthouse'
        : `another project already uses ${relay}`,
    )
    this.name = 'DuplicateRelayError'
  }
}

export function newProjectId(): string {
  const tail = Math.random().toString(36).slice(2, 10).padEnd(8, '0')
  return 'prj_' + tail
}

export const SWARM_KEY_HEX_RE = /^[0-9a-fA-F]{64}$/

export interface ProjectInput {
  name: string
  relayMultiaddr: string | null
  reputationFloor?: number
  connectionMode: ConnectionMode
  swarmKey: string | null
}

export function normalizeProject(p: Project): Project {
  return {
    ...p,
    connectionMode: p.connectionMode ?? 'public',
    swarmKey: p.swarmKey ?? null,
  }
}

export function validateProjectInput(
  existing: Project[],
  input: ProjectInput,
  editingId?: string,
): void {
  if (!input.name || !input.name.trim()) {
    throw new Error('Project name is required')
  }
  const floor = input.reputationFloor ?? -0.5
  if (floor < -1 || floor > 0) {
    throw new Error('Reputation floor must be between -1.0 and 0.0')
  }
  if (input.connectionMode === 'private') {
    if (!input.relayMultiaddr) {
      throw new Error('Private projects require a private relay multiaddr')
    }
    if (!input.swarmKey || !SWARM_KEY_HEX_RE.test(input.swarmKey)) {
      throw new Error('Private projects require a 64-character hex swarm key')
    }
  } else if (input.swarmKey) {
    throw new Error('Public projects cannot have a swarm key')
  }
  const conflict = existing.find(
    (p) => p.relayMultiaddr === input.relayMultiaddr && p.id !== editingId,
  )
  if (conflict) throw new DuplicateRelayError(input.relayMultiaddr)
}

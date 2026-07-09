import type { ChatSession } from '../types/chat';

const LIMIT = 50;

function keyFor(projectId: string): string {
  return `chat:sessions:${projectId}`;
}

export async function loadSessions(projectId: string): Promise<ChatSession[]> {
  try {
    const stored = await window.api.settings.get<ChatSession[]>(keyFor(projectId));
    return Array.isArray(stored) ? stored : [];
  } catch {
    return [];
  }
}

export async function saveSessions(
  projectId: string,
  sessions: ChatSession[],
): Promise<void> {
  // Sessions are newest-first; keep the newest LIMIT, not the oldest.
  const trimmed = sessions.slice(0, LIMIT);
  await window.api.settings.set(keyFor(projectId), trimmed);
}

export function newSession(): ChatSession {
  return {
    id: `chat_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    title: 'New chat',
    pinnedPeerId: null,
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
  };
}

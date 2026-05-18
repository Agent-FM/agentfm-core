import type { ChatSession } from '../types/chat';

const KEY = 'chat:sessions';
const LIMIT = 50;

export async function loadSessions(): Promise<ChatSession[]> {
  try {
    const stored = await window.api.settings.get<ChatSession[]>(KEY);
    return Array.isArray(stored) ? stored : [];
  } catch {
    return [];
  }
}

export async function saveSessions(sessions: ChatSession[]): Promise<void> {
  const trimmed = sessions.slice(-LIMIT);
  await window.api.settings.set(KEY, trimmed);
}

export function newSession(): ChatSession {
  return {
    id: `chat_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    title: 'New chat',
    pinnedPeerId: null,
    preferredModel: 'auto',
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
  };
}

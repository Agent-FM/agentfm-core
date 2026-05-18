import { useEffect, useRef, useState, useCallback } from 'react';
import { api, ApiError } from '../lib/api';
import { loadSessions, saveSessions, newSession } from '../lib/sessions';
import { useUIStore } from '../lib/store';
import type { ChatSession, ChatMessage } from '../types/chat';

export function useChat() {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const activeProjectId = useUIStore((s) => s.activeProjectId);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);
  useEffect(() => () => { mountedRef.current = false; }, []);

  useEffect(() => {
    if (!activeProjectId) return;
    setSessions([]);
    setActiveId(null);
    loadSessions(activeProjectId).then((s) => {
      setSessions(s);
      if (s.length > 0) setActiveId(s[0].id);
    });
  }, [activeProjectId]);

  useEffect(() => {
    if (!activeProjectId) return;
    if (sessions.length === 0) return;
    saveSessions(activeProjectId, sessions);
  }, [sessions, activeProjectId]);

  const active = sessions.find((s) => s.id === activeId);

  const createSession = useCallback(() => {
    const fresh = newSession();
    setSessions((prev) => [fresh, ...prev]);
    setActiveId(fresh.id);
  }, []);

  const selectSession = useCallback((id: string) => {
    setActiveId(id);
    setError(null);
  }, []);

  const deleteSession = useCallback(
    (id: string) => {
      setSessions((prev) => prev.filter((s) => s.id !== id));
      if (activeId === id) {
        setActiveId(() => {
          const remaining = sessions.filter((s) => s.id !== id);
          return remaining[0]?.id ?? null;
        });
      }
    },
    [activeId, sessions],
  );

  const updateActive = useCallback(
    (patch: Partial<ChatSession>) => {
      setSessions((prev) =>
        prev.map((s) =>
          s.id === activeId ? { ...s, ...patch, updatedAt: Date.now() } : s,
        ),
      );
    },
    [activeId],
  );

  const send = useCallback(
    async (content: string) => {
      if (!active || !content.trim() || streaming) return;
      setError(null);

      const userMsg: ChatMessage = {
        id: `msg_${Date.now()}`,
        role: 'user',
        content: content.trim(),
        timestamp: Date.now(),
      };
      const assistantMsg: ChatMessage = {
        id: `msg_${Date.now() + 1}`,
        role: 'assistant',
        content: '',
        timestamp: Date.now() + 1,
      };

      const isFirstMessage = active.messages.length === 0;
      const titleFromMsg = isFirstMessage
        ? content.trim().slice(0, 40) + (content.length > 40 ? '…' : '')
        : active.title;

      // Snapshot activeId for use in async callbacks
      const sessionId = activeId;

      // Append both messages; the assistant's content fills in as the stream arrives.
      setSessions((prev) =>
        prev.map((s) => {
          if (s.id !== sessionId) return s;
          return {
            ...s,
            title: titleFromMsg,
            messages: [...s.messages, userMsg, assistantMsg],
            updatedAt: Date.now(),
          };
        }),
      );

      setStreaming(true);
      const ctrl = new AbortController();
      abortRef.current = ctrl;

      const model = active.pinnedPeerId ?? active.preferredModel;
      const chatHistory = active.messages
        .map((m) => ({ role: m.role, content: m.content }))
        .concat({ role: 'user', content: content.trim() });

      try {
        const res = await api.chatCompletion(
          {
            model,
            messages: chatHistory,
            stream: true,
          },
          ctrl.signal,
        );

        if (!res.body) {
          setError('Backend returned no response body');
          setStreaming(false);
          return;
        }

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let assistantContent = '';
        let raterPeerId: string | undefined;

        // Throttled commit: streaming chats can emit hundreds of small SSE
        // lines a second. setSessions clones the full session tree on every
        // call, so committing per-line caused a render storm that froze
        // the UI. We accumulate into local state and rAF-commit at most
        // ~once per frame.
        let rafScheduled = false;
        const commit = () => {
          rafScheduled = false;
          if (!mountedRef.current) return;
          setSessions((prev) =>
            prev.map((s) => {
              if (s.id !== sessionId) return s;
              const msgs = s.messages.map((m) =>
                m.id === assistantMsg.id
                  ? { ...m, content: assistantContent, rater_peer_id: raterPeerId }
                  : m,
              );
              return { ...s, messages: msgs, updatedAt: Date.now() };
            }),
          );
        };
        const scheduleCommit = () => {
          if (rafScheduled) return;
          rafScheduled = true;
          requestAnimationFrame(commit);
        };

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() ?? '';

          let chunkChanged = false;
          for (const line of lines) {
            if (!line.startsWith('data: ')) continue;
            const data = line.slice(6).trim();
            if (data === '[DONE]') continue;
            try {
              const obj = JSON.parse(data);
              const delta = obj.choices?.[0]?.delta?.content;
              if (delta) {
                assistantContent += delta;
                chunkChanged = true;
              }
              if (!raterPeerId && obj.agentfm_peer_id) {
                raterPeerId = obj.agentfm_peer_id;
                chunkChanged = true;
              }
            } catch {
              /* skip malformed */
            }
          }
          if (chunkChanged) scheduleCommit();
        }

        // Final commit guarantees the last few characters land even if the
        // last rAF was already scheduled but not yet flushed.
        commit();
      } catch (err) {
        if ((err as Error).name !== 'AbortError') {
          const msg =
            err instanceof ApiError ? `${err.status}: ${err.message}` : (err as Error).message;
          setError(msg);
        }
      } finally {
        setStreaming(false);
        abortRef.current = null;
      }
    },
    [active, activeId, streaming],
  );

  const stop = useCallback(() => {
    abortRef.current?.abort();
    setStreaming(false);
  }, []);

  return {
    sessions,
    active,
    activeId,
    createSession,
    selectSession,
    deleteSession,
    updateActive,
    send,
    stop,
    streaming,
    error,
  };
}

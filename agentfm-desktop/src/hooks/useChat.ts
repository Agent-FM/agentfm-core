import { useEffect, useRef, useState, useCallback } from 'react';
import { api, ApiError } from '../lib/api';
import { loadSessions, saveSessions, newSession } from '../lib/sessions';
import { useUIStore } from '../lib/store';
import { usePeerIdentityCache } from '../lib/peerIdentityCache';
import { stripAnsi } from '../lib/ansi';
import type { ChatSession, ChatMessage } from '../types/chat';

export function useChat() {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const activeProjectId = useUIStore((s) => s.activeProjectId);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (!activeProjectId) return;
    let cancelled = false;
    setSessions([]);
    setActiveId(null);
    setLoaded(false);
    loadSessions(activeProjectId).then((s) => {
      if (cancelled) return;
      if (s.length === 0) {
        const fresh = newSession();
        setSessions([fresh]);
        setActiveId(fresh.id);
      } else {
        setSessions(s);
        setActiveId(s[0].id);
      }
      setLoaded(true);
    });
    return () => {
      cancelled = true;
    };
  }, [activeProjectId]);

  useEffect(() => {
    if (!activeProjectId) return;
    if (!loaded) return;
    saveSessions(activeProjectId, sessions);
  }, [sessions, activeProjectId, loaded]);

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
      setSessions((prev) => {
        const remaining = prev.filter((s) => s.id !== id);
        if (remaining.length === 0) {
          const fresh = newSession();
          setActiveId(fresh.id);
          return [fresh];
        }
        if (activeId === id) {
          setActiveId(remaining[0].id);
        }
        return remaining;
      });
    },
    [activeId],
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
      if (!active.pinnedPeerId) {
        setError('Pin an agent before sending a message.');
        return;
      }
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

      const model = active.pinnedPeerId;
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
        let taskId: string | undefined;

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
                  ? {
                      ...m,
                      content: assistantContent,
                      rater_peer_id: raterPeerId,
                      task_id: taskId,
                    }
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
                assistantContent += stripAnsi(delta);
                chunkChanged = true;
              }
              if (!raterPeerId && obj.agentfm_peer_id) {
                raterPeerId = obj.agentfm_peer_id;
                chunkChanged = true;
              }
              if (!taskId && obj.agentfm_task_id) {
                taskId = obj.agentfm_task_id;
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

        // Artifacts arrive on a separate libp2p channel after the SSE
        // stream closes. Poll the boss-side artifact directory for up to
        // 10s; if a zip lands, flip has_artifact on the assistant message
        // so MessageBubble can render the "Show in Finder" card. We also
        // drop a metadata sidecar so the Assets route can show the agent
        // name + prompt for chat-originated tasks instead of "Unknown".
        if (taskId) {
          const tid = taskId;
          const workerPeerId = raterPeerId ?? model;
          const cache = usePeerIdentityCache.getState().byPeerId[workerPeerId];
          const projectName = useUIStore.getState().activeProject()?.name;
          ;(async () => {
            for (let i = 0; i < 20; i++) {
              try {
                const exists = await window.api.app.checkArtifact(tid);
                if (exists) {
                  if (!mountedRef.current) return;
                  setSessions((prev) =>
                    prev.map((s) => {
                      if (s.id !== sessionId) return s;
                      const msgs = s.messages.map((m) =>
                        m.id === assistantMsg.id ? { ...m, has_artifact: true } : m,
                      );
                      return { ...s, messages: msgs, updatedAt: Date.now() };
                    }),
                  );
                  try {
                    await window.api.app.writeArtifactMeta(tid, {
                      prompt: content.trim(),
                      agentName: cache?.name ?? undefined,
                      agentDescription: cache?.description ?? undefined,
                      agentPeerId: workerPeerId,
                      projectName,
                    });
                  } catch {
                    // best-effort
                  }
                  return;
                }
              } catch {
                // ignore — keep polling
              }
              await new Promise((r) => setTimeout(r, 500));
            }
          })();
        }
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

import { useState, useRef, useCallback } from 'react';
import { api, ApiError } from '../lib/api';
import { stripAnsi } from '../lib/ansi';

export interface DispatchState {
  status: 'idle' | 'connecting' | 'streaming' | 'completed' | 'error';
  output: string;
  error?: string;
  taskId?: string;
  hasArtifact: boolean;
}

export interface DispatchMeta {
  agentName?: string;
  agentDescription?: string;
  projectName?: string;
}

export function useDispatch() {
  const [state, setState] = useState<DispatchState>({ status: 'idle', output: '', hasArtifact: false });
  const abortRef = useRef<AbortController | null>(null);

  const send = useCallback(async (workerId: string, prompt: string, meta?: DispatchMeta) => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    const taskId = `task_${Math.random().toString(36).slice(2, 10)}`;
    setState({ status: 'connecting', output: '', taskId, hasArtifact: false });

    try {
      const res = await api.execute({ worker_id: workerId, prompt, task_id: taskId }, ctrl.signal);
      if (!res.body) {
        setState({ status: 'error', output: '', error: 'empty response body', taskId, hasArtifact: false });
        return;
      }

      setState((s) => ({ ...s, status: 'streaming' }));
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true });
        buffer += chunk;
        // Strip [AGENTFM: ...] sentinel lines and ANSI escapes from the
        // displayed output (the chat path does the same via stripAnsi).
        const cleaned = stripAnsi(
          buffer
            .split('\n')
            .filter((line) => !line.trim().startsWith('[AGENTFM:'))
            .join('\n'),
        );
        setState((s) => ({ ...s, output: cleaned }));
      }

      setState((s) => ({ ...s, status: 'completed' }));

      // Poll for artifact for up to 10s. Once it appears, drop a metadata
      // sidecar so the Assets view can surface agent name + description
      // even after the agent has gone offline.
      for (let i = 0; i < 20; i++) {
        // Bail if this dispatch was superseded/reset, otherwise a stale loop
        // would write hasArtifact onto the NEXT dispatch's state.
        if (ctrl.signal.aborted) return;
        let exists = false;
        try {
          exists = await window.api.app.checkArtifact(taskId);
        } catch {
          // An IPC hiccup must not wipe the already-completed dispatch's
          // output via the outer catch, just stop polling.
          break;
        }
        if (exists) {
          if (ctrl.signal.aborted) return;
          setState((s) => ({ ...s, hasArtifact: true }));
          try {
            await window.api.app.writeArtifactMeta(taskId, {
              prompt,
              agentName: meta?.agentName,
              agentDescription: meta?.agentDescription,
              agentPeerId: workerId,
              projectName: meta?.projectName,
            });
          } catch {
            // Best-effort: a failed sidecar must not break the dispatch.
          }
          break;
        }
        await new Promise((r) => setTimeout(r, 500));
      }
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      const msg =
        err instanceof ApiError ? `${err.status}: ${err.message}` : (err as Error).message;
      setState({ status: 'error', output: '', error: msg, taskId, hasArtifact: false });
    }
  }, []);

  const reset = useCallback(() => {
    abortRef.current?.abort();
    setState({ status: 'idle', output: '', hasArtifact: false });
  }, []);

  return { state, send, reset };
}

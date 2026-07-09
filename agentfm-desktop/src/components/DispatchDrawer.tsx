import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { X, Check, Send, ExternalLink, FileArchive, Heart } from 'lucide-react';
import { useUIStore } from '../lib/store';
import { useWorkers } from '../lib/query';
import { useDispatch } from '../hooks/useDispatch';
import { usePeerIdentityCache } from '../lib/peerIdentityCache';
import { Button } from './primitives/Button';
import { Badge } from './primitives/Badge';
import { StreamingView } from './StreamingView';
import { shortenPeerID } from '../lib/peer';
import { displayName } from '../lib/displayName';

export function DispatchDrawer() {
  const isOpen = useUIStore((s) => s.isDispatchOpen);
  const target = useUIStore((s) => s.dispatchTarget);
  const close = useUIStore((s) => s.closeDispatch);
  const openFeedback = useUIStore((s) => s.openFeedback);

  const { data: workersData } = useWorkers(true);
  const worker = workersData?.agents.find((a) => a.peer_id === target);
  const cached = usePeerIdentityCache((s) =>
    target ? s.byPeerId[target] : undefined,
  );
  const activeProject = useUIStore((s) => s.activeProject());

  const [prompt, setPrompt] = useState('');
  const { state, send, reset } = useDispatch();
  const prevStatusRef = useRef(state.status);

  // Toast on terminal state transitions
  useEffect(() => {
    const prev = prevStatusRef.current;
    prevStatusRef.current = state.status;
    if (prev !== 'completed' && state.status === 'completed') {
      toast.success('Task completed');
    } else if (prev !== 'error' && state.status === 'error') {
      toast.error('Dispatch failed: ' + state.error);
    }
  }, [state.status, state.error]);

  useEffect(() => {
    if (!isOpen) {
      setPrompt('');
      reset();
    }
  }, [isOpen, reset]);

  function submit() {
    if (!worker || !prompt.trim()) return;
    send(worker.peer_id, prompt.trim(), {
      agentName: displayName(worker, cached),
      agentDescription: worker.description?.trim() || cached?.description,
      projectName: activeProject?.name,
    });
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      submit();
    }
  }

  // Esc to close
  useEffect(() => {
    function handler(e: KeyboardEvent) {
      if (e.key === 'Escape' && isOpen) close();
    }
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isOpen, close]);

  return (
    <AnimatePresence>
      {isOpen && worker && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="fixed inset-0 bg-bg-0/60 z-40"
            onClick={close}
          />
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className="fixed right-0 top-10 bottom-0 w-[52%] min-w-[480px] glass-strong rounded-l-sheet shadow-float overflow-auto p-4 z-50"
            onKeyDown={handleKey}
          >
            {/* Header */}
            <div className="flex items-start justify-between mb-4">
              <div>
                <div className="flex items-center gap-2">
                  <h2 className="text-lg font-semibold text-text-0">
                    {displayName(worker, cached)}
                  </h2>
                  {worker.model && (
                    <span className="text-sm text-text-2">{worker.model}</span>
                  )}
                </div>
                <div className="font-mono text-2xs text-text-2 mt-0.5">
                  {shortenPeerID(worker.peer_id, 12, 5)}
                </div>
                <div className="flex gap-1.5 mt-1.5">
                  <Badge
                    tone={
                      worker.honesty_score > 0.3
                        ? 'ok'
                        : worker.honesty_score < -0.5
                          ? 'bad'
                          : 'neutral'
                    }
                    mono
                  >
                    {worker.honesty_score >= 0 ? '+' : ''}
                    {worker.honesty_score.toFixed(2)}
                  </Badge>
                  <Badge
                    tone={worker.dispatch_allowed ? 'ok' : 'bad'}
                    title={worker.dispatch_refuse_reason}
                  >
                    {worker.dispatch_allowed ? (
                      <Check size={11} strokeWidth={2} aria-hidden />
                    ) : (
                      <X size={11} strokeWidth={2} aria-hidden />
                    )}
                    {worker.dispatch_allowed ? 'Allowed' : 'Refused'}
                  </Badge>
                </div>
              </div>
              <button onClick={close} className="text-text-2 hover:text-text-0 transition-colors">
                <X size={18} />
              </button>
            </div>

            {/* Prompt */}
            <div className="text-2xs font-medium text-text-2 mb-1.5">Prompt</div>
            <textarea
              autoFocus
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              aria-label="Task prompt"
              placeholder="Describe what you want the agent to do…"
              className="w-full min-h-[100px] glass-inset rounded-ctl p-3 text-sm text-text-0 outline-none focus:border-accent/60 resize-y"
              disabled={state.status === 'streaming' || state.status === 'connecting'}
            />
            <div className="text-2xs text-text-2 mt-1.5 flex justify-end items-center">
              <Button
                variant="primary"
                onClick={submit}
                disabled={
                  !prompt.trim() ||
                  state.status === 'streaming' ||
                  state.status === 'connecting'
                }
              >
                <Send size={12} />
                <span>
                  {state.status === 'streaming'
                    ? 'Streaming…'
                    : state.status === 'connecting'
                      ? 'Sending…'
                      : 'Send to agent'}
                </span>
              </Button>
            </div>

            {/* Live stream */}
            {(state.output || state.status !== 'idle') && (
              <>
                <div className="text-2xs font-medium text-text-2 mt-5 mb-1.5">
                  Live stream
                </div>
                <StreamingView
                  output={state.output}
                  streaming={state.status === 'streaming'}
                />
              </>
            )}

            {/* Error */}
            {state.status === 'error' && (
              <div className="mt-3 p-3 bg-bad/10 border border-bad/30 rounded-ctl text-xs text-bad">
                Stream error: {state.error}
              </div>
            )}

            {/* Artifact */}
            {state.hasArtifact && state.taskId && (
              <>
                <div className="text-2xs font-medium text-text-2 mt-5 mb-1.5">
                  Artifacts
                </div>
                <div className="glass-inset rounded-ctl p-2.5 flex items-center gap-3 text-xs">
                  <FileArchive size={14} strokeWidth={1.5} className="text-accent shrink-0" aria-hidden />
                  <span className="flex-1 font-mono tabular-nums">{state.taskId}.zip</span>
                  <Button onClick={() => window.api.app.openArtifact(state.taskId!)}>
                    <ExternalLink size={12} />
                    Show in Finder
                  </Button>
                </div>
              </>
            )}

            {/* Post-completion actions */}
            {state.status === 'completed' && (
              <div className="flex gap-2 mt-5 pt-4 border-t border-border-0">
                <Button onClick={() => { reset(); setPrompt(''); }}>Dispatch another</Button>
                <Button
                  variant="primary"
                  onClick={() => openFeedback(worker.peer_id, state.taskId!)}
                >
                  <Heart size={12} strokeWidth={1.5} aria-hidden />
                  <span>Leave feedback</span>
                </Button>
              </div>
            )}
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

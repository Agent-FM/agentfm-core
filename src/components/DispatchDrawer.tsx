import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { useUIStore } from '../lib/store';
import { useWorkers } from '../lib/query';
import { useDispatch } from '../hooks/useDispatch';
import { Button } from './primitives/Button';
import { HonestyBadge } from './HonestyBadge';
import { DispatchBadge } from './DispatchBadge';
import { StreamingView } from './StreamingView';
import { shortenPeerID } from '../lib/peer';

export function DispatchDrawer() {
  const isOpen = useUIStore((s) => s.isDispatchOpen);
  const target = useUIStore((s) => s.dispatchTarget);
  const close = useUIStore((s) => s.closeDispatch);
  const openFeedback = useUIStore((s) => s.openFeedback);

  const { data: workersData } = useWorkers(true);
  const worker = workersData?.agents.find((a) => a.peer_id === target);

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
    send(worker.peer_id, prompt.trim());
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
            className="fixed inset-0 bg-black/55 backdrop-blur-sm z-40"
            onClick={close}
          />
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 280, damping: 30 }}
            className="fixed right-0 top-10 bottom-0 w-[52%] min-w-[480px] bg-bg-1 border-l border-border-0 overflow-auto p-6 z-50"
            onKeyDown={handleKey}
          >
            {/* Header */}
            <div className="flex items-start justify-between mb-4">
              <div>
                <h2 className="text-base font-semibold text-text-0">
                  {worker.name}
                  {worker.model ? ` · ${worker.model}` : ''}
                </h2>
                <div className="font-mono text-[11px] text-text-2 mt-0.5">
                  {shortenPeerID(worker.peer_id, 12, 5)}
                </div>
                <div className="flex gap-1.5 mt-1.5">
                  <HonestyBadge score={worker.honesty_score} />
                  <DispatchBadge
                    allowed={worker.dispatch_allowed}
                    reason={worker.dispatch_refuse_reason}
                  />
                </div>
              </div>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">
                ✕
              </button>
            </div>

            {/* Prompt */}
            <div className="text-[11px] uppercase tracking-wider text-text-2 mb-1.5">Prompt</div>
            <textarea
              autoFocus
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Describe what you want the agent to do…"
              className="w-full min-h-[100px] bg-bg-0 border border-border-0 rounded-md p-3 text-sm text-text-0 outline-none focus:border-accent resize-y"
              disabled={state.status === 'streaming' || state.status === 'connecting'}
            />
            <div className="text-[11px] text-text-2 mt-1.5 flex justify-between items-center">
              <span>
                Press{' '}
                <kbd className="bg-bg-2 border border-border-0 border-b-2 px-1.5 py-0.5 rounded text-[10px] font-sans">
                  ⌘
                </kbd>{' '}
                +{' '}
                <kbd className="bg-bg-2 border border-border-0 border-b-2 px-1.5 py-0.5 rounded text-[10px] font-sans">
                  ↵
                </kbd>{' '}
                to send
              </span>
              <Button
                variant="primary"
                onClick={submit}
                disabled={
                  !prompt.trim() ||
                  state.status === 'streaming' ||
                  state.status === 'connecting'
                }
              >
                {state.status === 'streaming'
                  ? 'Streaming…'
                  : state.status === 'connecting'
                    ? 'Sending…'
                    : 'Send to agent'}
              </Button>
            </div>

            {/* Live stream */}
            {(state.output || state.status !== 'idle') && (
              <>
                <div className="text-[11px] uppercase tracking-wider text-text-2 mt-5 mb-1.5">
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
              <div className="mt-3 p-3 bg-rose-950/40 border border-rose-900/60 rounded-md text-xs text-rose-300">
                Stream error: {state.error}
              </div>
            )}

            {/* Artifact */}
            {state.hasArtifact && state.taskId && (
              <>
                <div className="text-[11px] uppercase tracking-wider text-text-2 mt-5 mb-1.5">
                  Artifacts
                </div>
                <div className="bg-bg-1 border border-border-0 rounded-md p-2.5 flex items-center gap-3 text-xs">
                  <span className="text-accent">📄</span>
                  <span className="flex-1 font-mono">{state.taskId}.zip</span>
                  <Button onClick={() => window.api.app.openArtifact(state.taskId!)}>
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
                  Leave feedback 💌
                </Button>
              </div>
            )}
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

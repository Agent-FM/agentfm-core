import { useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { useChat } from '../hooks/useChat';
import { SessionList } from '../components/chat/SessionList';
import { AgentPicker } from '../components/chat/AgentPicker';
import { MessageBubble } from '../components/chat/MessageBubble';
import { Composer } from '../components/chat/Composer';

export default function Chat() {
  const {
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
  } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [active?.messages]);

  // Toast on streaming errors
  const prevErrorRef = useRef<string | null>(null);
  useEffect(() => {
    if (error && error !== prevErrorRef.current) {
      toast.error('Chat stream failed: ' + error);
    }
    prevErrorRef.current = error;
  }, [error]);

  // If no sessions yet, auto-create one
  useEffect(() => {
    if (sessions.length === 0) createSession();
  }, [sessions, createSession]);

  if (!active) {
    return (
      <div className="flex flex-1 items-center justify-center text-text-2">
        Loading chat…
      </div>
    );
  }

  return (
    <div className="flex flex-1 h-full">
      <SessionList
        sessions={sessions}
        activeId={activeId}
        onSelect={selectSession}
        onNew={createSession}
        onDelete={deleteSession}
      />

      <div className="flex flex-col flex-1 bg-bg-0">
        <header className="border-b border-border-0 px-5 py-3 flex items-center gap-3">
          <AgentPicker
            pinnedPeerId={active.pinnedPeerId}
            preferredModel={active.preferredModel}
            onPin={(pid) => updateActive({ pinnedPeerId: pid })}
          />
        </header>

        <div
          ref={scrollRef}
          className="flex-1 overflow-auto px-5 py-5 flex flex-col gap-4"
        >
          <AnimatePresence initial={false}>
            {active.messages.map((m, i) => {
              const isLast = i === active.messages.length - 1;
              return (
                <MessageBubble
                  key={m.id}
                  msg={m}
                  streaming={streaming && isLast && m.role === 'assistant'}
                />
              );
            })}
          </AnimatePresence>
          {active.messages.length === 0 && (
            <div className="m-auto text-center text-text-2 text-sm">
              <div className="text-3xl mb-2">💬</div>
              Send a message to get started.
            </div>
          )}
          {error && (
            <div className="text-xs text-rose-400 bg-rose-950/40 border border-rose-900/60 rounded-md p-2.5">
              {error}
            </div>
          )}
        </div>

        <Composer onSend={send} onStop={stop} streaming={streaming} />
      </div>
    </div>
  );
}

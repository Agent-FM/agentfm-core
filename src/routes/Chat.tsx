import { useEffect, useRef } from 'react';
import { AnimatePresence } from 'framer-motion';
import { ArrowLeft } from 'lucide-react'
import { toast } from 'sonner';
import { useNavigate } from 'react-router-dom';
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
  const navigate = useNavigate();
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

      <div className="flex flex-col flex-1 min-w-0 overflow-hidden bg-bg-0">
        <header className="border-b border-border-0 px-5 py-3 flex items-center gap-3">
          <button onClick={() => navigate('/radar')}
            className="inline-flex items-center gap-1.5 text-xs text-text-2 hover:text-text-0">
            <ArrowLeft size={14} />
            <span>Radar</span>
          </button>
          <AgentPicker
            pinnedPeerId={active.pinnedPeerId}
            onPin={(pid) => updateActive({ pinnedPeerId: pid })}
          />
        </header>

        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto overflow-x-hidden px-5 py-5 flex flex-col gap-4 min-w-0"
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

        {!active.pinnedPeerId && (
          <div className="mx-5 mb-3 text-xs text-warn bg-warn/10 border border-warn/30 rounded-md px-3 py-2">
            Pin an agent above to start chatting. Use the picker to choose one from the online mesh.
          </div>
        )}
        <Composer onSend={send} onStop={stop} streaming={streaming} disabled={!active.pinnedPeerId} />
      </div>
    </div>
  );
}

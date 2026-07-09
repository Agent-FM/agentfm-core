import { useEffect, useRef } from 'react';
import { AnimatePresence } from 'framer-motion';
import { MessageSquare } from 'lucide-react';
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

      <div className="flex flex-col flex-1 min-w-0 overflow-hidden">
        <header className="glass-bar border-b border-border-0 px-3 py-1.5 flex items-center gap-3">
          <AgentPicker
            pinnedPeerId={active.pinnedPeerId}
            onPin={(pid) => updateActive({ pinnedPeerId: pid })}
          />
        </header>

        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto overflow-x-hidden bg-editor px-4 py-4 flex flex-col gap-3 min-w-0"
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
            <div className="m-auto flex flex-col items-center text-center">
              <MessageSquare size={28} strokeWidth={1.5} className="text-text-3 mb-2" aria-hidden="true" />
              <div className="text-sm text-text-3">Send a message to get started.</div>
            </div>
          )}
          {error && (
            <div className="text-xs text-bad bg-bad/10 border border-bad/30 rounded-ctl p-2.5">
              {error}
            </div>
          )}
        </div>

        {!active.pinnedPeerId && (
          <div className="mx-4 mb-2 text-xs text-warn bg-warn/10 border border-warn/30 rounded-ctl px-2.5 py-1.5">
            Pin an agent above to start chatting. Use the picker to choose one from the online mesh.
          </div>
        )}
        <Composer onSend={send} onStop={stop} streaming={streaming} disabled={!active.pinnedPeerId} />
      </div>
    </div>
  );
}

import { useState, useRef, useEffect } from 'react';
import { Button } from '../primitives/Button';

interface Props {
  onSend: (text: string) => void;
  onStop?: () => void;
  streaming?: boolean;
  disabled?: boolean;
}

export function Composer({ onSend, onStop, streaming, disabled }: Props) {
  const [text, setText] = useState('');
  const ref = useRef<HTMLTextAreaElement>(null);

  function submit() {
    if (!text.trim() || streaming || disabled) return;
    onSend(text);
    setText('');
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  // Auto-resize
  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto';
      ref.current.style.height = Math.min(ref.current.scrollHeight, 200) + 'px';
    }
  }, [text]);

  return (
    <div className="border-t border-border-0 p-3.5 flex gap-2.5 items-end">
      <textarea
        ref={ref}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={handleKey}
        placeholder="Type a message… (Shift+↵ for newline)"
        disabled={disabled}
        rows={1}
        className="flex-1 bg-bg-0 border border-border-0 rounded-md px-3 py-2.5 text-sm text-text-0 outline-none focus:border-accent resize-none disabled:opacity-50"
      />
      {streaming ? (
        <Button onClick={onStop}>Stop</Button>
      ) : (
        <Button
          variant="primary"
          onClick={submit}
          disabled={!text.trim() || disabled}
        >
          Send ↵
        </Button>
      )}
    </div>
  );
}

import { useState, useRef, useEffect } from 'react'
import { Send, Square } from 'lucide-react'
import { Button } from '../primitives/Button'

interface Props {
  onSend: (text: string) => void
  onStop?: () => void
  streaming?: boolean
  disabled?: boolean
}

export function Composer({ onSend, onStop, streaming, disabled }: Props) {
  const [text, setText] = useState('')
  const ref = useRef<HTMLTextAreaElement>(null)

  function submit() {
    if (!text.trim() || streaming || disabled) return
    onSend(text)
    setText('')
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit() }
  }

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto'
      ref.current.style.height = Math.min(ref.current.scrollHeight, 220) + 'px'
    }
  }, [text])

  return (
    <div className="border-t border-border-0 p-4 flex gap-3 items-end">
      <textarea
        ref={ref}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={handleKey}
        placeholder="Type a message… (Shift+↵ for newline)"
        disabled={disabled}
        rows={1}
        className="flex-1 bg-bg-0 border border-border-0 rounded-xl px-4 py-3 text-sm text-text-0 outline-none transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(34,211,238,.15)] resize-none disabled:opacity-50"
      />
      {streaming ? (
        <Button variant="danger" onClick={onStop}>
          <Square size={12} />
          <span>Stop</span>
        </Button>
      ) : (
        <Button variant="primary" onClick={submit} disabled={!text.trim() || disabled}>
          <Send size={12} />
          <span>Send</span>
        </Button>
      )}
    </div>
  )
}

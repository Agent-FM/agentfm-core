import { useState, useRef, useEffect } from 'react'
import { Button } from '../primitives/Button'
import { ArrowUp, Square } from 'lucide-react'

interface Props {
  onSend: (text: string) => void
  onStop?: () => void
  streaming?: boolean
  disabled?: boolean
}

export function Composer({ onSend, onStop, streaming, disabled }: Props) {
  const [text, setText] = useState('')
  const [focused, setFocused] = useState(false)
  const ref = useRef<HTMLTextAreaElement>(null)

  function submit() {
    if (!text.trim() || streaming || disabled) return
    onSend(text)
    setText('')
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  // Auto-grow the textarea up to a reasonable cap, then scroll.
  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto'
      ref.current.style.height = Math.min(ref.current.scrollHeight, 220) + 'px'
    }
  }, [text])

  const canSend = text.trim().length > 0 && !streaming && !disabled
  const lineCount = text === '' ? 0 : text.split('\n').length

  return (
    <div className="glass-bar border-t border-border-0 px-3 py-2.5">
      <div
        className={`relative flex items-end gap-2 bg-bg-well border rounded-ctl transition-colors duration-150 ${
          focused ? 'border-accent/50' : 'border-border-0 hover:border-border-1'
        }`}
      >
        <textarea
          ref={ref}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKey}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          placeholder="Type a message…   ⇧+↵ for newline"
          disabled={disabled}
          rows={1}
          className="flex-1 bg-transparent px-2.5 py-1.5 text-sm text-text-0 placeholder:text-text-2 outline-none resize-none disabled:opacity-50 min-h-[30px]"
        />
        <div className="flex items-center gap-2 px-1.5 py-1">
          {lineCount > 1 && (
            <span className="text-2xs font-mono text-text-2 select-none tabular-nums">
              {lineCount} lines
            </span>
          )}
          {streaming ? (
            <Button variant="danger" onClick={onStop}>
              <Square size={12} strokeWidth={1.5} />
              <span>Stop</span>
            </Button>
          ) : (
            <Button variant="primary" onClick={submit} disabled={!canSend}>
              <ArrowUp size={12} strokeWidth={1.5} />
              <span>Send</span>
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

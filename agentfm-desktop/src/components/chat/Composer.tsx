import { useState, useRef, useEffect } from 'react'
import { GradientButton } from '../primitives/GradientButton'
import { Zap, Square } from 'lucide-react'

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
    <div className="border-t border-border-0 px-4 py-4 bg-gradient-to-b from-transparent to-bg-1/40">
      <div
        className={`relative flex items-end gap-3 bg-bg-1 border rounded-2xl transition-all ${
          focused
            ? 'border-accent/60 shadow-[0_0_0_3px_rgba(34,211,238,.12),0_4px_24px_-8px_rgba(34,211,238,.35)]'
            : 'border-border-0 hover:border-border-1'
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
          className="flex-1 bg-transparent px-4 py-3 text-sm text-text-0 placeholder:text-text-3 outline-none resize-none disabled:opacity-50 min-h-[48px]"
        />
        <div className="flex items-center gap-2 px-2 py-2">
          {lineCount > 1 && (
            <span className="text-2xs font-mono text-text-3 select-none">
              {lineCount} lines
            </span>
          )}
          {streaming ? (
            <button
              onClick={onStop}
              className="inline-flex items-center justify-center gap-1.5 px-4 py-2.5
                rounded-xl font-semibold text-[13px] border border-bad/40
                bg-bad/10 text-bad hover:bg-bad/20 hover:border-bad/60 transition-colors"
            >
              <Square size={12} />
              <span>Stop</span>
            </button>
          ) : (
            <GradientButton onClick={submit} disabled={!canSend}>
              <Zap size={12} />
              <span>Send</span>
            </GradientButton>
          )}
        </div>
      </div>
    </div>
  )
}

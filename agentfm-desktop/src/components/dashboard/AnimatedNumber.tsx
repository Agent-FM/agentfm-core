import { useEffect, useRef, useState } from 'react'
import { animate } from 'framer-motion'

const TWEEN_MS = 500
const SKIP_RATIO = 10

interface Props {
  value: number
  format?: (n: number) => string
}

export function AnimatedNumber({ value, format }: Props) {
  const [display, setDisplay] = useState(value)
  const lastValueRef = useRef(value)

  useEffect(() => {
    const prev = lastValueRef.current
    lastValueRef.current = value
    if (prev === value) return

    const big =
      Math.abs(value - prev) > Math.max(Math.abs(prev), 1) * SKIP_RATIO
    if (big) {
      setDisplay(value)
      return
    }
    const controls = animate(prev, value, {
      duration: TWEEN_MS / 1000,
      ease: 'easeOut',
      onUpdate: (v) => setDisplay(v),
    })
    return () => controls.stop()
  }, [value])

  const fmt = format ?? ((n: number) => Math.round(n).toString())
  return <span className="tabular-nums">{fmt(display)}</span>
}

import { useEffect, useRef, useState } from 'react'

export interface SparkLineProps {
  values: number[]
  height: number
  color: string
  /** Fixed pixel width. Omit to fill the container (measured via ResizeObserver). */
  width?: number
}

export function SparkLine({ values, height, color, width }: SparkLineProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
  const [measured, setMeasured] = useState(width ?? 0)
  const w = width ?? measured
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null)

  useEffect(() => {
    if (width != null || typeof ResizeObserver === 'undefined') return
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const cw = entries[0]?.contentRect.width
      if (cw && cw > 0) setMeasured(Math.floor(cw))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [width])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || w <= 0) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    canvas.width = w * dpr
    canvas.height = height * dpr
    canvas.style.width = `${w}px`
    canvas.style.height = `${height}px`
    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, w, height)

    if (values.length === 0) {
      setTip(null)
      return
    }

    let min = values[0]
    let max = values[0]
    for (const v of values) {
      if (v < min) min = v
      if (v > max) max = v
    }
    const range = max - min || 1
    const stepX = values.length > 1 ? w / (values.length - 1) : w

    ctx.beginPath()
    let lastX = 0
    let lastY = 0
    for (let i = 0; i < values.length; i++) {
      const x = i * stepX
      const y = height - ((values[i] - min) / range) * (height - 2) - 1
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
      lastX = x
      lastY = y
    }
    ctx.strokeStyle = color
    ctx.lineWidth = 1.2
    ctx.stroke()

    ctx.lineTo(w, height)
    ctx.lineTo(0, height)
    ctx.closePath()
    ctx.globalAlpha = 0.1
    ctx.fillStyle = color
    ctx.fill()
    ctx.globalAlpha = 1

    setTip(values.length >= 2 ? { x: lastX, y: lastY } : null)
  }, [values, w, height, color])

  return (
    <div ref={wrapRef} className="relative" style={{ width: width ?? '100%', height }}>
      <canvas ref={canvasRef} />
      {tip && (
        <span
          aria-hidden
          className="absolute w-1.5 h-1.5 rounded-full pointer-events-none"
          style={{
            background: color,
            left: tip.x - 3,
            top: tip.y - 3,
          }}
        />
      )}
    </div>
  )
}

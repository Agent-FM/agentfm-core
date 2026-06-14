import { useEffect, useRef, useState } from 'react'

export interface SparkLineProps {
  values: number[]
  width: number
  height: number
  color: string
}

export function SparkLine({ values, width, height, color }: SparkLineProps) {
  const ref = useRef<HTMLCanvasElement>(null)
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null)

  useEffect(() => {
    const canvas = ref.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    canvas.width = width * dpr
    canvas.height = height * dpr
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, width, height)

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
    const stepX = values.length > 1 ? width / (values.length - 1) : width

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

    ctx.lineTo(width, height)
    ctx.lineTo(0, height)
    ctx.closePath()
    ctx.fillStyle = 'rgba(34,211,238,0.08)'
    ctx.fill()

    setTip(values.length >= 2 ? { x: lastX, y: lastY } : null)
  }, [values, width, height, color])

  return (
    <div className="relative" style={{ width, height }}>
      <canvas ref={ref} />
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

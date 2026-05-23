import { useEffect, useRef } from 'react'
import uPlot from 'uplot'
import type { Options, AlignedData } from 'uplot'

export interface UPlotChartProps {
  data: AlignedData
  height: number
  series: { label: string; color: string }[]
}

export function UPlotChart({ data, height, series }: UPlotChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<uPlot | null>(null)

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    const opts: Options = {
      width: el.clientWidth || 600,
      height,
      legend: { show: false },
      cursor: { show: false },
      scales: { x: { time: true } },
      axes: [
        { stroke: '#64748b', grid: { stroke: 'rgba(148,163,184,0.08)' } },
        { stroke: '#64748b', grid: { stroke: 'rgba(148,163,184,0.08)' } },
      ],
      series: [
        {},
        ...series.map((s) => ({
          label: s.label,
          stroke: s.color,
          width: 1.5,
          fill: s.color + '22',
        })),
      ],
    }

    const chart = new uPlot(opts, data, el)
    chartRef.current = chart

    const ro = new ResizeObserver(() => {
      if (chartRef.current && el.clientWidth > 0) {
        chartRef.current.setSize({ width: el.clientWidth, height })
      }
    })
    ro.observe(el)

    return () => {
      ro.disconnect()
      chart.destroy()
      chartRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [series.length, height])

  useEffect(() => {
    if (chartRef.current) {
      chartRef.current.setData(data)
    }
  }, [data])

  return <div ref={containerRef} style={{ width: '100%', height }} />
}

import { useEffect, useRef } from 'react'
import { getApiBaseURL } from '../lib/api'
import { parseMetrics } from '../lib/promParse'
import { useMetricsStore } from '../lib/metricsStore'

const FAST_INTERVAL_MS = 2_000
const SLOW_INTERVAL_MS = 10_000
const ERROR_THRESHOLD = 3

/**
 * Polls the boss /metrics endpoint while the document is visible and the
 * hook is mounted. Pauses on `visibilitychange:hidden`. Switches to a 10s
 * backoff after 3 consecutive errors; returns to 2s on first success.
 *
 * Call this from inside a route component that should drive polling (only
 * Dashboard today). Other routes that don't need /metrics should not call
 * it, the boss `/metrics` endpoint is unrelated to the always-on
 * `/api/workers` poll handled by useWorkerHistory.
 */
export function useMetricsPoll(): void {
  const errorsRef = useRef(0)
  const cancelledRef = useRef(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pushBoss = useMetricsStore((s) => s.pushBoss)

  useEffect(() => {
    cancelledRef.current = false

    async function tick() {
      if (cancelledRef.current) return
      if (document.visibilityState !== 'visible') {
        // Null the (already-fired) timer id so onVisibility restarts polling
        // when the tab becomes visible again. Without this, timerRef stays
        // truthy and !timerRef.current never re-triggers tick().
        timerRef.current = null
        return
      }
      try {
        const res = await fetch(`${getApiBaseURL()}/metrics`)
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const text = await res.text()
        const samples = parseMetrics(text)
        pushBoss(Date.now(), samples)
        errorsRef.current = 0
      } catch {
        errorsRef.current++
      }
      if (cancelledRef.current) return
      const next =
        errorsRef.current >= ERROR_THRESHOLD ? SLOW_INTERVAL_MS : FAST_INTERVAL_MS
      timerRef.current = setTimeout(tick, next)
    }

    function onVisibility() {
      if (document.visibilityState === 'visible' && !timerRef.current) {
        tick()
      }
    }

    document.addEventListener('visibilitychange', onVisibility)
    tick()

    return () => {
      cancelledRef.current = true
      if (timerRef.current) clearTimeout(timerRef.current)
      timerRef.current = null
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [pushBoss])
}

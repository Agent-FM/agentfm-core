import { useEffect, useState } from 'react'

export interface BackendStatus {
  ok: boolean
  online_workers: number
  consecutiveFailures: number
}

export function useBackend() {
  const [status, setStatus] = useState<BackendStatus>({
    ok: false,
    online_workers: 0,
    consecutiveFailures: 0,
  })

  useEffect(() => {
    let cancelled = false
    let failures = 0

    async function probe() {
      try {
        const res = await window.api.backend.health()
        if (cancelled) return
        if (res && (res as { ok: boolean }).ok) {
          failures = 0
          setStatus({
            ok: true,
            online_workers: (res as { online_workers: number }).online_workers ?? 0,
            consecutiveFailures: 0,
          })
        } else {
          failures++
          setStatus({ ok: false, online_workers: 0, consecutiveFailures: failures })
        }
      } catch {
        failures++
        if (!cancelled) setStatus({ ok: false, online_workers: 0, consecutiveFailures: failures })
      }
    }

    probe()
    const id = setInterval(probe, 3000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  return status
}

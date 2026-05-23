import { useEffect } from 'react'
import { useWorkers } from '../lib/query'
import { useMetricsStore } from '../lib/metricsStore'
import type { WorkerProfile } from '../types/api'

/**
 * Continuously captures a per-peer history of CPU%, GPU%, RAM-free (GB),
 * and queue depth from the existing useWorkers React Query poll.
 *
 * Call this from App.tsx so the buffer is always populated regardless of
 * which route is visible — when a user navigates to PeerView, charts
 * already have several minutes of history.
 */
export function useWorkerHistory(): void {
  const { data } = useWorkers(true)
  const pushPeer = useMetricsStore((s) => s.pushPeer)

  useEffect(() => {
    if (!data) return
    const now = Date.now()
    for (const w of data.agents as WorkerProfile[]) {
      if (!w.online) continue
      pushPeer(w.peer_id, now, {
        cpu: w.cpu_usage_pct ?? 0,
        gpu: w.gpu_usage_pct ?? 0,
        ram: w.ram_free_gb ?? 0,
        queue: w.current_tasks ?? 0,
      })
    }
  }, [data, pushPeer])
}

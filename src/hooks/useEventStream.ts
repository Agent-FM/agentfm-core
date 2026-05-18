import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { openEventStream } from '../lib/sse'
import { qk } from '../lib/query'

export function useEventStream() {
  const qc = useQueryClient()

  useEffect(() => {
    const handle = openEventStream('http://127.0.0.1:8080/v1/events', {
      onEvent: (type, data) => {
        switch (type) {
          case 'worker_online':
          case 'worker_offline':
            qc.invalidateQueries({ queryKey: ['workers'] })
            break
          case 'entry_appended': {
            const subject: string | undefined = (data as { subject_peer_id?: string })
              ?.subject_peer_id
            if (subject) {
              qc.invalidateQueries({ queryKey: qk.peer(subject) })
              qc.invalidateQueries({ queryKey: ['peer-log', subject] })
            }
            break
          }
          case 'equivocator_marked': {
            const pid: string | undefined = (data as { peer_id?: string })?.peer_id
            if (pid) {
              qc.invalidateQueries({ queryKey: qk.peer(pid) })
              qc.invalidateQueries({ queryKey: ['workers'] })
            }
            break
          }
        }
      },
    })
    return () => handle.close()
  }, [qc])
}

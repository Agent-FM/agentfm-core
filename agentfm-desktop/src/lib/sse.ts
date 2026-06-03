export interface SseListener {
  onEvent: (type: string, data: unknown) => void
  onError?: (err: Event) => void
  onOpen?: () => void
}

export interface SseHandle {
  close: () => void
}

const KNOWN_EVENTS = [
  'worker_online',
  'worker_offline',
  'entry_appended',
  'equivocator_marked',
] as const

export function openEventStream(url: string, listener: SseListener): SseHandle {
  let source: EventSource | null = null
  let retry = 1000
  let closed = false

  function connect() {
    if (closed) return
    source = new EventSource(url)

    source.addEventListener('open', () => {
      retry = 1000
      listener.onOpen?.()
    })

    for (const evt of KNOWN_EVENTS) {
      source.addEventListener(evt, (ev: MessageEvent) => {
        try {
          const data = ev.data ? JSON.parse(ev.data) : null
          listener.onEvent(evt, data)
        } catch {
          listener.onEvent(evt, ev.data)
        }
      })
    }

    source.onerror = (err) => {
      listener.onError?.(err)
      source?.close()
      if (closed) return
      setTimeout(connect, Math.min(retry, 30_000))
      retry *= 2
    }
  }

  connect()

  return {
    close: () => {
      closed = true
      source?.close()
    },
  }
}

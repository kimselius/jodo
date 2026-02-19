export function createSSE(
  url: string,
  onMessage: (data: unknown) => void,
  onStatus?: (connected: boolean) => void
) {
  let source: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let stopped = false

  function connect() {
    if (stopped) return

    source = new EventSource(url)

    source.onopen = () => {
      onStatus?.(true)
    }

    source.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        onMessage(data)
      } catch {
        // ignore parse errors
      }
    }

    source.onerror = () => {
      onStatus?.(false)
      source?.close()
      if (!stopped) {
        reconnectTimer = setTimeout(connect, 3000)
      }
    }
  }

  connect()

  return {
    close() {
      stopped = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      source?.close()
      onStatus?.(false)
    }
  }
}

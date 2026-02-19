import { ref, readonly } from 'vue'

export interface WSEvent {
  type: 'chat' | 'memory' | 'growth' | 'timeline' | 'heartbeat' | string
  data: unknown
}

type WSHandler = (event: WSEvent) => void

const connected = ref(false)
const handlers = new Set<WSHandler>()
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0
const MAX_RECONNECT_DELAY = 30_000

function getWSUrl() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/api/ws`
}

function connect() {
  if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
    return
  }

  ws = new WebSocket(getWSUrl())

  ws.onopen = () => {
    connected.value = true
    reconnectAttempts = 0
  }

  ws.onmessage = (ev) => {
    try {
      const event = JSON.parse(ev.data) as WSEvent
      handlers.forEach((fn) => fn(event))
    } catch {
      // ignore parse errors
    }
  }

  ws.onclose = () => {
    connected.value = false
    scheduleReconnect()
  }

  ws.onerror = () => {
    ws?.close()
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), MAX_RECONNECT_DELAY)
  reconnectAttempts++
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, delay)
}

// Auto-connect on first import
connect()

/**
 * Subscribe to WebSocket events. Returns an unsubscribe function.
 */
export function onWSEvent(handler: WSHandler): () => void {
  handlers.add(handler)
  return () => handlers.delete(handler)
}

/**
 * Composable for WebSocket connection status.
 */
export function useWebSocket() {
  return {
    connected: readonly(connected),
  }
}

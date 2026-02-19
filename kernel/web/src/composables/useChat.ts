import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '@/lib/api'
import { onWSEvent, useWebSocket } from './useWebSocket'
import type { ChatMessage } from '@/types/chat'

export function useChat() {
  const messages = ref<ChatMessage[]>([])
  const loading = ref(true)
  const sending = ref(false)
  const { connected } = useWebSocket()

  async function loadHistory() {
    loading.value = true
    try {
      const data = await api.getMessages({ last: '50' })
      messages.value = data.messages || []
    } catch (e) {
      console.error('Failed to load chat history:', e)
    } finally {
      loading.value = false
    }
  }

  // Listen for real-time chat messages via WebSocket
  let unsub: (() => void) | null = null

  function startWS() {
    unsub = onWSEvent((event) => {
      if (event.type === 'chat') {
        const chatMsg = event.data as ChatMessage
        if (!messages.value.some(m => m.id === chatMsg.id)) {
          messages.value.push(chatMsg)
        }
      }
    })
  }

  async function send(text: string) {
    const trimmed = text.trim()
    if (!trimmed || sending.value) return

    sending.value = true
    try {
      await api.sendMessage(trimmed)
    } catch (e) {
      console.error('Failed to send message:', e)
    } finally {
      sending.value = false
    }
  }

  onMounted(() => {
    loadHistory()
    startWS()
  })

  onUnmounted(() => {
    unsub?.()
  })

  return { messages, loading, sending, connected, send, loadHistory }
}

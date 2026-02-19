import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { api } from '@/lib/api'
import { createSSE } from '@/lib/sse'
import type { ChatMessage } from '@/types/chat'

export function useChat() {
  const messages = ref<ChatMessage[]>([])
  const loading = ref(true)
  const sending = ref(false)
  const connected = ref(false)
  let sse: ReturnType<typeof createSSE> | null = null

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

  function startSSE() {
    sse = createSSE(
      '/api/chat/stream',
      (msg) => {
        const chatMsg = msg as ChatMessage
        if (!messages.value.some(m => m.id === chatMsg.id)) {
          messages.value.push(chatMsg)
        }
      },
      (status) => {
        connected.value = status
      }
    )
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
    startSSE()
  })

  onUnmounted(() => {
    sse?.close()
  })

  return { messages, loading, sending, connected, send, loadHistory }
}

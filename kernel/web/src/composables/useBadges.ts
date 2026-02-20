import { reactive, watch } from 'vue'
import { useRoute } from 'vue-router'
import { onWSEvent } from './useWebSocket'
import { api } from '@/lib/api'

// Maps WS event types to route paths
const eventToRoute: Record<string, string> = {
  chat: '/',
  memory: '/memories',
  library: '/library',
  inbox: '/logs',
}

// Reactive badge counts — keyed by route path
const badges = reactive<Record<string, number>>({
  '/': 0,
  '/memories': 0,
  '/gallas': 0,
  '/logs': 0,
  '/library': 0,
})

// Track whether the module has been initialized
let initialized = false
let currentPath = '/'

function init() {
  if (initialized) return
  initialized = true

  // Load initial unread chat count from DB
  loadUnreadChat()

  // Increment badge counts on incoming WS events
  onWSEvent((event) => {
    // Handle growth events — only badge galla updates, not routine log entries
    if (event.type === 'growth') {
      const data = event.data as { event?: string; galla?: number }
      if (data.galla !== undefined && !data.event) {
        if (currentPath !== '/gallas') badges['/gallas']++
      }
      return
    }

    const route = eventToRoute[event.type]
    if (route && route in badges) {
      // Don't badge the page the user is currently viewing
      // (except /logs — its badge is cleared by the Inbox tab, not by navigation)
      if (route === currentPath && route !== '/logs') return
      // Only badge Jodo's messages, not the human's own
      if (event.type === 'chat') {
        const data = event.data as { source?: string }
        if (data.source === 'human') return
      }
      badges[route]++
    }
  })
}

async function loadUnreadChat() {
  try {
    const data = await api.getMessages({ unread: 'true', source: 'jodo' })
    const count = data.messages?.length ?? 0
    if (count > 0 && currentPath !== '/') {
      badges['/'] = count
    }
  } catch { /* setup may not be complete yet */ }
}

/**
 * Clear badge for a specific route. Called by composables that ack their own data.
 */
export function clearBadge(path: string) {
  if (path in badges) badges[path] = 0
}

/**
 * Composable for badge counts. Clears the badge for the current route.
 */
export function useBadges() {
  init()

  const route = useRoute()

  // Clear badge when user navigates to a route.
  // /logs is excluded — its badge is cleared by the Inbox tab via clearBadge().
  watch(
    () => route.path,
    (path) => {
      currentPath = path
      if (path in badges && path !== '/logs') {
        badges[path] = 0
      }
    },
    { immediate: true }
  )

  return { badges }
}

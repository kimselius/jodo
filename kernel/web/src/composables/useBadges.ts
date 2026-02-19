import { reactive, watch } from 'vue'
import { useRoute } from 'vue-router'
import { onWSEvent } from './useWebSocket'

// Maps WS event types to route paths
const eventToRoute: Record<string, string> = {
  chat: '/',
  memory: '/memories',
  timeline: '/timeline',
}

// Reactive badge counts — keyed by route path
const badges = reactive<Record<string, number>>({
  '/': 0,
  '/memories': 0,
  '/growth': 0,
  '/logs': 0,
  '/timeline': 0,
})

// Track whether the module has been initialized
let initialized = false
let currentPath = '/'

function init() {
  if (initialized) return
  initialized = true

  // Increment badge counts on incoming WS events
  onWSEvent((event) => {
    // Handle growth events — split between galla updates and log events
    if (event.type === 'growth') {
      const data = event.data as { event?: string; galla?: number }
      if (data.event) {
        // Log event (from handleLog)
        if (currentPath !== '/logs') badges['/logs']++
      } else {
        // Galla update (from handleGallaPost)
        if (currentPath !== '/growth') badges['/growth']++
      }
      return
    }

    const route = eventToRoute[event.type]
    if (route && route in badges) {
      // Don't badge the page the user is currently viewing
      if (route === currentPath) return
      // Only badge Jodo's messages, not the human's own
      if (event.type === 'chat') {
        const data = event.data as { source?: string }
        if (data.source === 'human') return
      }
      badges[route]++
    }
  })
}

/**
 * Composable for badge counts. Clears the badge for the current route.
 */
export function useBadges() {
  init()

  const route = useRoute()

  // Clear badge when user navigates to a route
  watch(
    () => route.path,
    (path) => {
      currentPath = path
      if (path in badges) {
        badges[path] = 0
      }
    },
    { immediate: true }
  )

  return { badges }
}

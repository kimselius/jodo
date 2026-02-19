import { reactive, watch } from 'vue'
import { useRoute } from 'vue-router'
import { onWSEvent } from './useWebSocket'

// Maps WS event types to route paths
const eventToRoute: Record<string, string> = {
  chat: '/',
  memory: '/memories',
  growth: '/growth',
  timeline: '/timeline',
}

// Reactive badge counts — keyed by route path
const badges = reactive<Record<string, number>>({
  '/': 0,
  '/memories': 0,
  '/growth': 0,
  '/timeline': 0,
})

// Track whether the module has been initialized
let initialized = false

function init() {
  if (initialized) return
  initialized = true

  // Increment badge counts on incoming WS events
  onWSEvent((event) => {
    const route = eventToRoute[event.type]
    if (route && route in badges) {
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
      if (path in badges) {
        badges[path] = 0
      }
    },
    { immediate: true }
  )

  return { badges }
}

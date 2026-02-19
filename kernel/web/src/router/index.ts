import { createRouter, createWebHistory } from 'vue-router'
import { api } from '@/lib/api'

let setupChecked = false
let isSetupComplete = true // assume complete until checked

export function markSetupComplete() {
  isSetupComplete = true
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup',
      name: 'setup',
      component: () => import('@/views/SetupWizard.vue'),
      meta: { setupRoute: true },
    },
    {
      path: '/',
      name: 'chat',
      component: () => import('@/views/ChatView.vue'),
    },
    {
      path: '/status',
      name: 'status',
      component: () => import('@/views/StatusView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue'),
    },
    {
      path: '/memories',
      name: 'memories',
      component: () => import('@/views/MemoriesView.vue'),
    },
    {
      path: '/library',
      name: 'library',
      component: () => import('@/views/LibraryView.vue'),
    },
    {
      path: '/growth',
      name: 'growth',
      component: () => import('@/views/GrowthView.vue'),
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('@/views/LogView.vue'),
    },
    {
      path: '/inbox',
      name: 'inbox',
      component: () => import('@/views/InboxView.vue'),
    },
    {
      path: '/timeline',
      name: 'timeline',
      component: () => import('@/views/TimelineView.vue'),
    },
  ],
})

router.beforeEach(async (to) => {
  if (!setupChecked) {
    setupChecked = true
    try {
      const status = await api.getSetupStatus()
      isSetupComplete = status.setup_complete
    } catch {
      isSetupComplete = true
    }
  }

  if (!isSetupComplete && !to.meta.setupRoute) {
    return { name: 'setup' }
  }

  if (isSetupComplete && to.meta.setupRoute) {
    return { name: 'chat' }
  }
})

export default router

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
      path: '/gallas',
      name: 'gallas',
      component: () => import('@/views/GallasView.vue'),
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('@/views/LogsView.vue'),
    },
    // Redirects for old bookmarks
    { path: '/growth', redirect: '/gallas' },
    { path: '/inbox', redirect: '/logs' },
    { path: '/timeline', redirect: '/logs' },
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

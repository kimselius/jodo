import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'chat',
      component: () => import('@/views/ChatView.vue')
    },
    {
      path: '/status',
      name: 'status',
      component: () => import('@/views/StatusView.vue')
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue')
    },
    {
      path: '/memories',
      name: 'memories',
      component: () => import('@/views/MemoriesView.vue')
    },
    {
      path: '/growth',
      name: 'growth',
      component: () => import('@/views/GrowthView.vue')
    },
    {
      path: '/timeline',
      name: 'timeline',
      component: () => import('@/views/TimelineView.vue')
    }
  ]
})

export default router

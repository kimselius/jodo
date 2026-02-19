<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useStatus } from '@/composables/useStatus'
import Badge from '@/components/ui/Badge.vue'
import Separator from '@/components/ui/Separator.vue'

const route = useRoute()
const { status } = useStatus(15_000)

const jodoName = computed(() => status.value?.jodo?.current_git_tag || 'Jodo')
const jodoStatus = computed(() => status.value?.jodo?.status || 'unknown')
const jodoPhase = computed(() => status.value?.jodo?.phase || '')
const jodoGalla = computed(() => status.value?.jodo?.galla ?? 0)

const statusVariant = computed(() => {
  switch (jodoStatus.value) {
    case 'running': return 'success'
    case 'starting': return 'warning'
    case 'unhealthy': return 'warning'
    case 'dead': return 'destructive'
    case 'rebirthing': return 'accent'
    default: return 'secondary'
  }
})

// Activity label for the footer indicator
const activityLabel = computed(() => {
  const s = jodoStatus.value
  if (s === 'dead') return 'Offline'
  if (s === 'starting') return 'Starting...'
  if (s === 'rebirthing') return 'Rebirthing...'
  if (s === 'unhealthy') return 'Unhealthy'
  // running — use phase
  switch (jodoPhase.value) {
    case 'thinking': return `Thinking (g${jodoGalla.value})`
    case 'sleeping': return 'Sleeping'
    case 'booting': return 'Booting...'
    default: return 'Running'
  }
})

// Dot color class for the footer indicator
const activityDotClass = computed(() => {
  const s = jodoStatus.value
  if (s === 'dead') return 'bg-red-500'
  if (s === 'unhealthy') return 'bg-amber-500'
  if (s === 'starting' || s === 'rebirthing') return 'bg-amber-500 animate-pulse'
  if (jodoPhase.value === 'thinking') return 'bg-green-500 animate-pulse'
  if (jodoPhase.value === 'sleeping') return 'bg-blue-400'
  return 'bg-green-500'
})

const navItems = [
  { path: '/', name: 'Chat', icon: 'chat' },
  { path: '/status', name: 'Status', icon: 'status' },
  { path: '/memories', name: 'Memories', icon: 'memories' },
  { path: '/growth', name: 'Growth', icon: 'growth' },
  { path: '/settings', name: 'Settings', icon: 'settings' },
  { path: '/timeline', name: 'Timeline', icon: 'timeline' },
]

defineEmits<{ close: [] }>()
</script>

<template>
  <div class="flex h-full flex-col bg-card">
    <!-- Header -->
    <div class="p-4">
      <div class="flex items-center gap-3">
        <div class="flex h-9 w-9 items-center justify-center rounded-full bg-primary/15">
          <span class="text-primary text-lg font-bold">J</span>
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-sm font-semibold truncate">Jodo</h2>
          <Badge :variant="statusVariant" class="mt-0.5">
            {{ jodoStatus }}
          </Badge>
        </div>
      </div>
    </div>

    <Separator />

    <!-- Navigation -->
    <nav class="flex-1 p-2">
      <RouterLink
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        @click="$emit('close')"
        :class="[
          'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
          route.path === item.path
            ? 'bg-secondary text-foreground font-medium'
            : 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'
        ]"
      >
        <svg v-if="item.icon === 'chat'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
        <svg v-else-if="item.icon === 'status'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
        <svg v-else-if="item.icon === 'settings'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <svg v-else-if="item.icon === 'memories'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
        <svg v-else-if="item.icon === 'growth'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
        </svg>
        <svg v-else-if="item.icon === 'timeline'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ item.name }}
      </RouterLink>
    </nav>

    <Separator />

    <!-- Activity indicator -->
    <div class="p-3">
      <div class="flex items-center gap-2 px-1">
        <span :class="['inline-block h-2 w-2 rounded-full shrink-0', activityDotClass]" />
        <span class="text-xs text-muted-foreground truncate">{{ activityLabel }}</span>
      </div>
    </div>
  </div>
</template>

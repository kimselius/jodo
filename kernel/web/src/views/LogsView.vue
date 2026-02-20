<script setup lang="ts">
import { ref, watch } from 'vue'
import LLMCallsTab from '@/components/logs/LLMCallsTab.vue'
import EventsTab from '@/components/logs/EventsTab.vue'
import InboxTab from '@/components/logs/InboxTab.vue'
import { useBadges, clearBadge } from '@/composables/useBadges'

const { badges } = useBadges()
const activeTab = ref<'llm' | 'events' | 'inbox'>('llm')

// Clear inbox badge when the Inbox tab is viewed
watch(activeTab, (tab) => {
  if (tab === 'inbox') clearBadge('/logs')
}, { immediate: true })

const tabs = [
  { key: 'llm' as const, label: 'LLM Calls' },
  { key: 'events' as const, label: 'Events' },
  { key: 'inbox' as const, label: 'Inbox' },
]
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-3xl mx-auto p-4">
      <h1 class="text-lg font-semibold mb-4">Logs</h1>

      <!-- Tab bar -->
      <div class="flex gap-1 mb-4 border-b border-border">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          @click="activeTab = tab.key"
          :class="[
            'px-3 py-2 text-sm transition-colors border-b-2 -mb-px',
            activeTab === tab.key
              ? 'border-primary text-foreground font-medium'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          ]"
        >
          {{ tab.label }}
          <span
            v-if="tab.key === 'inbox' && badges['/logs'] > 0 && activeTab !== 'inbox'"
            class="ml-1.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-[10px] font-medium text-primary-foreground"
          >
            {{ badges['/logs'] > 99 ? '99+' : badges['/logs'] }}
          </span>
        </button>
      </div>

      <!-- Tab content -->
      <KeepAlive>
        <LLMCallsTab v-if="activeTab === 'llm'" />
        <EventsTab v-else-if="activeTab === 'events'" />
        <InboxTab v-else-if="activeTab === 'inbox'" />
      </KeepAlive>
    </div>
  </div>
</template>

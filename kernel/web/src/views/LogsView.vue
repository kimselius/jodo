<script setup lang="ts">
import { ref } from 'vue'
import LLMCallsTab from '@/components/logs/LLMCallsTab.vue'
import EventsTab from '@/components/logs/EventsTab.vue'
import InboxTab from '@/components/logs/InboxTab.vue'

const activeTab = ref<'llm' | 'events' | 'inbox'>('llm')

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

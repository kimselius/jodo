<script setup lang="ts">
import { ref } from 'vue'
import GallaTimeline from '@/components/growth/GallaTimeline.vue'
import GrowthEntry from '@/components/growth/GrowthEntry.vue'
import Button from '@/components/ui/Button.vue'
import { useGrowth } from '@/composables/useGrowth'

const tab = ref<'growth' | 'log'>('growth')
const { events, loading, error, load } = useGrowth()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <!-- Tabs -->
      <div class="flex gap-1 mb-4 border-b border-border">
        <button
          @click="tab = 'growth'"
          :class="[
            'px-3 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
            tab === 'growth'
              ? 'border-primary text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          ]"
        >
          Growth
        </button>
        <button
          @click="tab = 'log'"
          :class="[
            'px-3 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
            tab === 'log'
              ? 'border-primary text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          ]"
        >
          Log
        </button>
      </div>

      <!-- Growth tab (galla timeline) -->
      <GallaTimeline v-if="tab === 'growth'" />

      <!-- Log tab (existing events) -->
      <div v-else>
        <div class="flex items-center justify-between mb-4">
          <h1 class="text-lg font-semibold">Event Log</h1>
          <Button variant="ghost" size="sm" @click="load">Refresh</Button>
        </div>

        <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

        <div v-if="loading" class="flex items-center justify-center py-12">
          <span class="text-sm text-muted-foreground">Loading...</span>
        </div>

        <div v-else-if="events.length === 0" class="text-center py-12">
          <p class="text-sm text-muted-foreground">No events yet.</p>
        </div>

        <div v-else>
          <GrowthEntry
            v-for="ev in events"
            :key="ev.id"
            :event="ev"
          />
        </div>
      </div>
    </div>
  </div>
</template>

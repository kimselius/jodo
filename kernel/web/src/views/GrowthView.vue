<script setup lang="ts">
import { useGrowth } from '@/composables/useGrowth'
import GrowthEntry from '@/components/growth/GrowthEntry.vue'
import Button from '@/components/ui/Button.vue'

const { events, loading, error, load } = useGrowth()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <h1 class="text-lg font-semibold">Growth Log</h1>
        <Button variant="ghost" size="sm" @click="load">Refresh</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else-if="events.length === 0" class="text-center py-12">
        <p class="text-sm text-muted-foreground">No growth events yet.</p>
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
</template>

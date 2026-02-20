<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import GrowthEntry from '@/components/growth/GrowthEntry.vue'
import Button from '@/components/ui/Button.vue'
import { useGrowth } from '@/composables/useGrowth'

const { events, loading, error, load } = useGrowth()
const container = ref<HTMLElement>()

watch(
  () => events.value.length,
  async () => {
    await nextTick()
    if (container.value) {
      container.value.scrollTop = container.value.scrollHeight
    }
  }
)
</script>

<template>
  <div ref="container">
    <div class="flex items-center justify-between mb-3">
      <p class="text-xs text-muted-foreground">System events: boot, rebirth, restart, rollback...</p>
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
</template>

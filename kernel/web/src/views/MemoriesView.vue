<script setup lang="ts">
import { useMemories } from '@/composables/useMemories'
import MemoryCard from '@/components/memories/MemoryCard.vue'
import Button from '@/components/ui/Button.vue'

const { memories, total, loading, error, load, loadMore, hasMore } = useMemories()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h1 class="text-lg font-semibold">Memories</h1>
          <p class="text-xs text-muted-foreground mt-0.5">{{ total }} stored</p>
        </div>
        <Button variant="ghost" size="sm" @click="load(true)">Refresh</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

      <div v-if="loading && memories.length === 0" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else-if="memories.length === 0" class="text-center py-12">
        <p class="text-sm text-muted-foreground">No memories yet. Jodo stores memories as it learns.</p>
      </div>

      <div v-else class="space-y-3">
        <MemoryCard
          v-for="mem in memories"
          :key="mem.id"
          :memory="mem"
        />

        <div v-if="hasMore()" class="flex justify-center pt-2 pb-4">
          <Button variant="ghost" size="sm" :disabled="loading" @click="loadMore">
            {{ loading ? 'Loading...' : 'Load more' }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

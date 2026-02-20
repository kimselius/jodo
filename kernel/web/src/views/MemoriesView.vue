<script setup lang="ts">
import { ref } from 'vue'
import { useMemories } from '@/composables/useMemories'
import { useMemorySearch } from '@/composables/useMemorySearch'
import MemoryCard from '@/components/memories/MemoryCard.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'

const { memories, total, loading, error, load, loadMore, hasMore } = useMemories()
const { query, results, searching, searchError, search, clearSearch } = useMemorySearch()

const isSearchMode = ref(false)

function handleSearch() {
  if (query.value.trim()) {
    isSearchMode.value = true
    search()
  }
}

function handleClear() {
  isSearchMode.value = false
  clearSearch()
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') handleSearch()
  if (e.key === 'Escape') handleClear()
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h1 class="text-lg font-semibold">Memories</h1>
          <p class="text-xs text-muted-foreground mt-0.5">{{ total }} stored</p>
        </div>
        <Button variant="ghost" size="sm" @click="isSearchMode ? handleClear() : load(true)">
          {{ isSearchMode ? 'Back to list' : 'Refresh' }}
        </Button>
      </div>

      <!-- Search bar -->
      <div class="flex gap-2 mb-4">
        <Input
          v-model="query"
          placeholder="Search memories..."
          @keydown="handleKeydown"
          class="flex-1"
        />
        <Button variant="secondary" size="sm" :disabled="searching || !query.trim()" @click="handleSearch">
          {{ searching ? 'Searching...' : 'Search' }}
        </Button>
      </div>

      <p v-if="error || searchError" class="text-sm text-destructive mb-4">{{ error || searchError }}</p>

      <!-- Search results -->
      <template v-if="isSearchMode">
        <div v-if="searching" class="flex items-center justify-center py-12">
          <span class="text-sm text-muted-foreground">Searching...</span>
        </div>

        <div v-else-if="results.length === 0" class="text-center py-12">
          <p class="text-sm text-muted-foreground">No matching memories found.</p>
        </div>

        <div v-else class="space-y-3">
          <p class="text-xs text-muted-foreground">{{ results.length }} result{{ results.length !== 1 ? 's' : '' }}</p>
          <div
            v-for="result in results"
            :key="result.id"
            class="rounded-lg border border-border p-3"
          >
            <p class="text-sm whitespace-pre-wrap break-words">{{ result.content }}</p>
            <div class="flex items-center gap-2 mt-2 flex-wrap">
              <span
                v-for="tag in result.tags"
                :key="tag"
                class="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5 text-[10px] font-medium text-secondary-foreground"
              >{{ tag }}</span>
              <span class="text-[10px] text-muted-foreground ml-auto">
                {{ Math.round(result.similarity * 100) }}% match
              </span>
            </div>
          </div>
        </div>
      </template>

      <!-- Browse list -->
      <template v-else>
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
      </template>
    </div>
  </div>
</template>

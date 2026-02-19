<script setup lang="ts">
import { useHistory } from '@/composables/useHistory'
import CommitEntryComp from '@/components/timeline/CommitEntry.vue'
import Button from '@/components/ui/Button.vue'

const { commits, loading, error, load } = useHistory()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <h1 class="text-lg font-semibold">Timeline</h1>
        <Button variant="ghost" size="sm" @click="load">Refresh</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else-if="commits.length === 0" class="text-center py-12">
        <p class="text-sm text-muted-foreground">No commits yet.</p>
      </div>

      <div v-else>
        <CommitEntryComp
          v-for="commit in commits"
          :key="commit.hash"
          :commit="commit"
        />
      </div>
    </div>
  </div>
</template>

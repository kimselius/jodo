<script setup lang="ts">
import { useGallas } from '@/composables/useGallas'
import GallaCard from '@/components/growth/GallaCard.vue'
import Button from '@/components/ui/Button.vue'

const { gallas, loading, error, load } = useGallas()
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-lg font-semibold">Growth</h1>
      <Button variant="ghost" size="sm" @click="load">Refresh</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <span class="text-sm text-muted-foreground">Loading...</span>
    </div>

    <div v-else-if="gallas.length === 0" class="text-center py-12">
      <p class="text-sm text-muted-foreground">No gallas yet. Waiting for Jodo to wake up...</p>
    </div>

    <div v-else class="space-y-3">
      <GallaCard
        v-for="g in gallas"
        :key="g.id"
        :galla="g"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useStatus } from '@/composables/useStatus'
import StatusCard from '@/components/status/StatusCard.vue'
import BudgetCard from '@/components/status/BudgetCard.vue'
import Button from '@/components/ui/Button.vue'
import { api } from '@/lib/api'
import { ref } from 'vue'

const { status, budget, error, refresh } = useStatus()
const restarting = ref(false)

async function handleRestart() {
  if (!confirm('Restart Jodo? This will kill the current process and reboot seed.py.')) return
  restarting.value = true
  try {
    await api.restart()
    setTimeout(refresh, 3000)
  } catch (e) {
    console.error('Restart failed:', e)
  } finally {
    restarting.value = false
  }
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4 space-y-4">
      <div class="flex items-center justify-between">
        <h1 class="text-lg font-semibold">Status</h1>
        <Button variant="destructive" size="sm" :disabled="restarting" @click="handleRestart">
          {{ restarting ? 'Restarting...' : 'Restart Jodo' }}
        </Button>
      </div>

      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

      <StatusCard v-if="status" :status="status" />
      <BudgetCard v-if="budget" :budget="budget" />

      <div v-if="!status && !error" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>
    </div>
  </div>
</template>

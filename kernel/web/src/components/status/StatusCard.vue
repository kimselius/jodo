<script setup lang="ts">
import { computed } from 'vue'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import { formatDuration } from '@/lib/utils'
import type { StatusResponse } from '@/types/status'

const props = defineProps<{ status: StatusResponse }>()

const jodoVariant = computed(() => {
  switch (props.status.jodo.status) {
    case 'running': return 'success'
    case 'starting': return 'warning'
    case 'unhealthy': return 'warning'
    case 'dead': return 'destructive'
    case 'rebirthing': return 'accent'
    default: return 'secondary'
  }
})
</script>

<template>
  <Card class="p-4">
    <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">System</h3>

    <div class="space-y-3">
      <!-- Jodo -->
      <div class="flex items-center justify-between">
        <span class="text-sm">Jodo</span>
        <Badge :variant="jodoVariant">{{ status.jodo.status }}</Badge>
      </div>
      <div class="flex items-center justify-between text-xs text-muted-foreground">
        <span>Uptime</span>
        <span>{{ formatDuration(status.jodo.uptime_seconds) }}</span>
      </div>
      <div class="flex items-center justify-between text-xs text-muted-foreground">
        <span>PID</span>
        <span class="font-mono">{{ status.jodo.pid || '—' }}</span>
      </div>
      <div class="flex items-center justify-between text-xs text-muted-foreground">
        <span>Restarts today</span>
        <span>{{ status.jodo.restarts_today }}</span>
      </div>
      <div class="flex items-center justify-between text-xs text-muted-foreground">
        <span>Health check</span>
        <span :class="status.jodo.health_check_ok ? 'text-success' : 'text-destructive'">
          {{ status.jodo.health_check_ok ? 'OK' : 'Failing' }}
        </span>
      </div>

      <!-- Kernel -->
      <div class="border-t border-border pt-3 mt-3">
        <div class="flex items-center justify-between">
          <span class="text-sm">Kernel</span>
          <Badge variant="success">{{ status.kernel.status }}</Badge>
        </div>
        <div class="flex items-center justify-between text-xs text-muted-foreground mt-2">
          <span>Uptime</span>
          <span>{{ formatDuration(status.kernel.uptime_seconds) }}</span>
        </div>
      </div>

      <!-- Database -->
      <div class="border-t border-border pt-3 mt-3">
        <div class="flex items-center justify-between">
          <span class="text-sm">Database</span>
          <Badge :variant="status.database.status === 'connected' ? 'success' : 'destructive'">
            {{ status.database.status }}
          </Badge>
        </div>
        <div class="flex items-center justify-between text-xs text-muted-foreground mt-2">
          <span>Memories</span>
          <span>{{ status.database.memories_stored }}</span>
        </div>
      </div>
    </div>
  </Card>
</template>

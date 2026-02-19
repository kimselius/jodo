<script setup lang="ts">
import { ref, watch } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import { api } from '@/lib/api'
import type { KernelSettings } from '@/types/setup'

const props = defineProps<{
  kernel: KernelSettings
}>()

const emit = defineEmits<{ saved: [] }>()

const form = ref<KernelSettings>({
  health_check_interval: 10,
  max_restart_attempts: 3,
  log_level: 'info',
  audit_log_path: '',
  external_url: '',
})
const saving = ref(false)
const error = ref<string | null>(null)

watch(() => props.kernel, (k) => {
  if (k) {
    form.value = { ...k }
  }
}, { immediate: true })

async function save() {
  saving.value = true
  error.value = null
  try {
    await api.updateSettingsKernel(form.value)
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">External URL</label>
        <Input v-model="form.external_url" placeholder="http://1.2.3.4:8080" />
        <p class="text-xs text-muted-foreground mt-1">How Jodo's VPS reaches this kernel.</p>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-sm font-medium mb-1.5 block">Health Check Interval (s)</label>
          <Input v-model.number="form.health_check_interval" type="number" />
        </div>
        <div>
          <label class="text-sm font-medium mb-1.5 block">Max Restart Attempts</label>
          <Input v-model.number="form.max_restart_attempts" type="number" />
        </div>
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">Log Level</label>
        <Input v-model="form.log_level" placeholder="info" />
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">Audit Log Path</label>
        <Input v-model="form.audit_log_path" placeholder="/var/log/jodo-audit.jsonl" />
      </div>
    </Card>

    <p class="text-xs text-muted-foreground">
      Some kernel settings require a restart to take effect.
    </p>

    <div class="flex justify-end">
      <Button @click="save" :disabled="saving">
        {{ saving ? 'Saving...' : 'Save Kernel Settings' }}
      </Button>
    </div>
  </div>
</template>

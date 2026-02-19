<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import type { ProvisionStep } from '@/types/setup'

defineProps<{
  brainPath: string
  provisioning: boolean
  provisionSteps: ProvisionStep[]
  provisioned: boolean
  error: string | null
}>()

const emit = defineEmits<{
  'update:brainPath': [value: string]
  provision: []
  next: []
  back: []
}>()
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">Server Setup</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Prepare the server for Jodo. This will create the brain directory and initialize a git repository.
      </p>
    </div>

    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">Brain Path</label>
        <Input
          :model-value="brainPath"
          @update:model-value="emit('update:brainPath', $event)"
          placeholder="/opt/jodo/brain"
        />
        <p class="text-xs text-muted-foreground mt-1">
          Directory where Jodo will store its code and data.
        </p>
      </div>

      <Button
        @click="$emit('provision')"
        :disabled="provisioning || !brainPath"
        variant="secondary"
      >
        {{ provisioning ? 'Provisioning...' : provisioned ? 'Re-provision Server' : 'Provision Server' }}
      </Button>
    </Card>

    <!-- Provision results -->
    <Card v-if="provisionSteps.length > 0" class="p-4 space-y-3">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Provision Results</h3>
      <div
        v-for="step in provisionSteps"
        :key="step.name"
        class="flex items-start gap-2 text-sm"
      >
        <span :class="step.ok ? 'text-green-500' : 'text-destructive'" class="mt-0.5 shrink-0">
          {{ step.ok ? '\u2713' : '\u2717' }}
        </span>
        <div class="min-w-0">
          <span class="font-medium">{{ step.name }}</span>
          <pre v-if="step.output" class="text-xs text-muted-foreground mt-0.5 font-mono whitespace-pre-wrap break-all">{{ step.output }}</pre>
        </div>
      </div>
    </Card>

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')" :disabled="!provisioned">Next</Button>
    </div>
  </div>
</template>

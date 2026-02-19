<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'

const model = defineModel<string>()

defineEmits<{
  next: []
  back: []
}>()
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">Kernel URL</h2>
      <p class="text-sm text-muted-foreground mt-1">
        The URL that Jodo's VPS uses to reach this kernel. This is how Jodo communicates back to you.
      </p>
    </div>

    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">External URL</label>
        <Input v-model="model" placeholder="http://1.2.3.4:8080" />
        <p class="text-xs text-muted-foreground mt-1.5">
          This must be reachable from Jodo's VPS. Use the public IP of this server, not <code>localhost</code>.
        </p>
      </div>
    </Card>

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')" :disabled="!model">Next</Button>
    </div>
  </div>
</template>

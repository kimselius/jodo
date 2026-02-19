<script setup lang="ts">
import { ref, watch } from 'vue'
import Input from '@/components/ui/Input.vue'
import Textarea from '@/components/ui/Textarea.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import type { Genesis, IdentityUpdate } from '@/types/genesis'

const props = defineProps<{
  genesis: Genesis
  saving: boolean
}>()

const emit = defineEmits<{
  save: [update: IdentityUpdate]
}>()

const name = ref(props.genesis.identity.name)
const purpose = ref(props.genesis.purpose)

watch(() => props.genesis, (g) => {
  name.value = g.identity.name
  purpose.value = g.purpose
})

function handleSave() {
  const update: IdentityUpdate = {}
  if (name.value !== props.genesis.identity.name) update.name = name.value
  if (purpose.value !== props.genesis.purpose) update.purpose = purpose.value
  if (Object.keys(update).length > 0) {
    emit('save', update)
  }
}

const hasChanges = () =>
  name.value !== props.genesis.identity.name ||
  purpose.value !== props.genesis.purpose
</script>

<template>
  <Card class="p-4">
    <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-4">Identity</h3>

    <div class="space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">Name</label>
        <Input v-model="name" placeholder="Jodo" />
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">Purpose</label>
        <Textarea
          v-model="purpose"
          placeholder="What is Jodo's purpose?"
          class="min-h-[120px]"
        />
        <p class="text-xs text-muted-foreground mt-1">
          This defines Jodo's personality and goals. Jodo reads this every galla.
        </p>
      </div>

      <Button
        :disabled="saving || !hasChanges()"
        @click="handleSave"
      >
        {{ saving ? 'Saving...' : 'Save Changes' }}
      </Button>
    </div>
  </Card>
</template>

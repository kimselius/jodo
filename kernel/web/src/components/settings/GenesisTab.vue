<script setup lang="ts">
import { ref, watch } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Textarea from '@/components/ui/Textarea.vue'
import Button from '@/components/ui/Button.vue'
import { api } from '@/lib/api'
import type { Genesis } from '@/types/genesis'

const props = defineProps<{
  genesis: Genesis
}>()

const emit = defineEmits<{ saved: [] }>()

const form = ref({
  name: '',
  purpose: '',
  survival_instincts: [] as string[],
  first_tasks: [] as string[],
  hints: [] as string[],
})

const saving = ref(false)
const error = ref<string | null>(null)

watch(() => props.genesis, (g) => {
  if (g) {
    form.value.name = g.identity.name
    form.value.purpose = g.purpose
    form.value.survival_instincts = [...g.survival_instincts]
    form.value.first_tasks = [...g.first_tasks]
    form.value.hints = [...g.hints]
  }
}, { immediate: true })

function addItem(list: string[]) {
  list.push('')
}

function removeItem(list: string[], index: number) {
  list.splice(index, 1)
}

async function save() {
  saving.value = true
  error.value = null
  try {
    await api.updateSettingsGenesis({
      name: form.value.name,
      purpose: form.value.purpose,
      survival_instincts: form.value.survival_instincts.filter(s => s.trim()),
      first_tasks: form.value.first_tasks.filter(s => s.trim()),
      hints: form.value.hints.filter(s => s.trim()),
      capabilities_api: props.genesis.capabilities?.kernel_api || {},
      capabilities_local: props.genesis.capabilities?.local || [],
    })
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
        <label class="text-sm font-medium mb-1.5 block">Name</label>
        <Input v-model="form.name" placeholder="Jodo" />
      </div>
      <div>
        <label class="text-sm font-medium mb-1.5 block">Purpose</label>
        <Textarea v-model="form.purpose" class="min-h-[120px]" />
        <p class="text-xs text-muted-foreground mt-1">Jodo reads this every galla.</p>
      </div>
    </Card>

    <Card class="p-4 space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Survival Instincts</h3>
        <Button size="sm" variant="ghost" @click="addItem(form.survival_instincts)">+ Add</Button>
      </div>
      <div v-for="(_, i) in form.survival_instincts" :key="i" class="flex gap-2">
        <Input v-model="form.survival_instincts[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(form.survival_instincts, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <Card class="p-4 space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">First Tasks</h3>
        <Button size="sm" variant="ghost" @click="addItem(form.first_tasks)">+ Add</Button>
      </div>
      <div v-for="(_, i) in form.first_tasks" :key="i" class="flex gap-2">
        <Input v-model="form.first_tasks[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(form.first_tasks, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <Card class="p-4 space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Hints</h3>
        <Button size="sm" variant="ghost" @click="addItem(form.hints)">+ Add</Button>
      </div>
      <div v-for="(_, i) in form.hints" :key="i" class="flex gap-2">
        <Input v-model="form.hints[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(form.hints, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <div class="flex justify-end">
      <Button @click="save" :disabled="saving">
        {{ saving ? 'Saving...' : 'Save Genesis' }}
      </Button>
    </div>
  </div>
</template>

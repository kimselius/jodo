<script setup lang="ts">
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Textarea from '@/components/ui/Textarea.vue'
import Button from '@/components/ui/Button.vue'
import type { GenesisSetup } from '@/types/setup'

const props = defineProps<{
  genesis: GenesisSetup
}>()

defineEmits<{
  next: []
  back: []
}>()

function addItem(list: string[]) {
  list.push('')
}

function removeItem(list: string[], index: number) {
  list.splice(index, 1)
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">Genesis</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Define Jodo's identity and initial instructions. These shape who Jodo becomes.
      </p>
    </div>

    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">Name</label>
        <Input v-model="genesis.name" placeholder="Jodo" />
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">Purpose</label>
        <Textarea v-model="genesis.purpose" class="min-h-[120px]" />
        <p class="text-xs text-muted-foreground mt-1">Jodo reads this every galla (life cycle). It defines who they are.</p>
      </div>
    </Card>

    <Card class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-medium">Survival Instincts</h3>
        <Button size="sm" variant="ghost" @click="addItem(genesis.survival_instincts)">+ Add</Button>
      </div>
      <div v-for="(_, i) in genesis.survival_instincts" :key="i" class="flex gap-2">
        <Input v-model="genesis.survival_instincts[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(genesis.survival_instincts, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <Card class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-medium">First Tasks</h3>
        <Button size="sm" variant="ghost" @click="addItem(genesis.first_tasks)">+ Add</Button>
      </div>
      <div v-for="(_, i) in genesis.first_tasks" :key="i" class="flex gap-2">
        <Input v-model="genesis.first_tasks[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(genesis.first_tasks, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <Card class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-medium">Hints</h3>
        <Button size="sm" variant="ghost" @click="addItem(genesis.hints)">+ Add</Button>
      </div>
      <div v-for="(_, i) in genesis.hints" :key="i" class="flex gap-2">
        <Input v-model="genesis.hints[i]" class="flex-1" />
        <Button size="icon" variant="ghost" @click="removeItem(genesis.hints, i)" class="shrink-0 text-muted-foreground hover:text-destructive">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Button>
      </div>
    </Card>

    <div class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button @click="$emit('next')">Next</Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import type { ProviderSetup, GenesisSetup } from '@/types/setup'
import { api } from '@/lib/api'

const props = defineProps<{
  vps: { host: string; sshUser: string }
  kernelUrl: string
  providers: ProviderSetup[]
  genesis: GenesisSetup
  birthing: boolean
}>()

const emit = defineEmits<{
  birth: []
  back: []
}>()

const router = useRouter()
const birthStatus = ref('')

const enabledProviders = props.providers.filter(p => p.enabled)
const totalModels = enabledProviders.reduce((sum, p) => sum + p.models.length, 0)

async function handleBirth() {
  birthStatus.value = 'Saving configuration...'
  emit('birth')

  // Poll status until Jodo starts (or timeout)
  let attempts = 0
  const poll = setInterval(async () => {
    attempts++
    try {
      const setupRes = await api.getSetupStatus()
      if (setupRes.setup_complete) {
        birthStatus.value = 'Jodo is being born...'
      }

      // Check if Jodo has started responding
      if (setupRes.setup_complete && attempts > 5) {
        try {
          const statusRes = await api.getStatus()
          if (statusRes.jodo.status !== 'dead') {
            clearInterval(poll)
            birthStatus.value = 'Jodo is alive!'
            setTimeout(() => router.push('/'), 1500)
            return
          }
        } catch {
          // Status endpoint not ready yet, keep polling
        }
      }
    } catch {
      // Setup endpoint returned error (might be restarting)
    }

    if (attempts > 60) {
      clearInterval(poll)
      birthStatus.value = 'Birth is taking longer than expected. Check the kernel logs.'
    }
  }, 2000)
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-lg font-semibold">Review & Birth</h2>
      <p class="text-sm text-muted-foreground mt-1">
        Review your configuration before bringing Jodo to life.
      </p>
    </div>

    <Card class="p-4 space-y-3">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">VPS Connection</h3>
      <div class="text-sm">
        <span class="text-muted-foreground">Host:</span>
        <span class="ml-2 font-mono">{{ vps.sshUser }}@{{ vps.host }}</span>
      </div>
      <div class="text-sm">
        <span class="text-muted-foreground">Kernel URL:</span>
        <span class="ml-2 font-mono">{{ kernelUrl }}</span>
      </div>
    </Card>

    <Card class="p-4 space-y-3">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">LLM Providers</h3>
      <div class="text-sm">
        {{ enabledProviders.length }} provider{{ enabledProviders.length !== 1 ? 's' : '' }} enabled,
        {{ totalModels }} model{{ totalModels !== 1 ? 's' : '' }} configured
      </div>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="p in enabledProviders"
          :key="p.name"
          class="text-xs bg-muted px-2 py-1 rounded capitalize"
        >
          {{ p.name }}
          <span v-if="p.monthly_budget > 0" class="text-muted-foreground">${{ p.monthly_budget }}/mo</span>
        </span>
      </div>
    </Card>

    <Card class="p-4 space-y-3">
      <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">Genesis</h3>
      <div class="text-sm">
        <span class="text-muted-foreground">Name:</span>
        <span class="ml-2 font-medium">{{ genesis.name }}</span>
      </div>
      <p class="text-sm text-muted-foreground line-clamp-3">{{ genesis.purpose }}</p>
    </Card>

    <!-- Birth section -->
    <div v-if="!birthing && !birthStatus" class="flex justify-between pt-4">
      <Button variant="ghost" @click="$emit('back')">Back</Button>
      <Button size="lg" @click="handleBirth">
        Birth Jodo
      </Button>
    </div>

    <!-- Birth in progress -->
    <div v-else class="text-center py-8 space-y-4">
      <div class="flex justify-center">
        <div class="h-12 w-12 rounded-full border-4 border-primary/30 border-t-primary animate-spin" />
      </div>
      <p class="text-sm text-muted-foreground">{{ birthStatus || 'Preparing...' }}</p>
    </div>
  </div>
</template>

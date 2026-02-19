<script setup lang="ts">
import { ref } from 'vue'
import { useGenesis } from '@/composables/useGenesis'
import IdentityForm from '@/components/settings/IdentityForm.vue'
import Card from '@/components/ui/Card.vue'
import type { IdentityUpdate } from '@/types/genesis'

const { genesis, loading, saving, error, updateIdentity } = useGenesis()

const saved = ref(false)

async function handleSave(update: IdentityUpdate) {
  const ok = await updateIdentity(update)
  if (ok) {
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  }
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4 space-y-4">
      <h1 class="text-lg font-semibold">Settings</h1>

      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <p v-if="saved" class="text-sm text-success">Saved successfully.</p>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <template v-else-if="genesis">
        <IdentityForm :genesis="genesis" :saving="saving" @save="handleSave" />

        <!-- Survival Instincts (read-only) -->
        <Card class="p-4">
          <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
            Survival Instincts
          </h3>
          <ul class="space-y-2">
            <li
              v-for="(instinct, i) in genesis.survival_instincts"
              :key="i"
              class="text-sm text-muted-foreground flex gap-2"
            >
              <span class="text-primary shrink-0">&#8226;</span>
              <span>{{ instinct }}</span>
            </li>
          </ul>
        </Card>

        <!-- Version info -->
        <Card class="p-4">
          <h3 class="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
            Info
          </h3>
          <div class="space-y-2 text-sm text-muted-foreground">
            <div class="flex justify-between">
              <span>Version</span>
              <span class="font-mono">{{ genesis.identity.version }}</span>
            </div>
          </div>
        </Card>
      </template>
    </div>
  </div>
</template>

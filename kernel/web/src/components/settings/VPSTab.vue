<script setup lang="ts">
import { ref } from 'vue'
import Card from '@/components/ui/Card.vue'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import { api } from '@/lib/api'
import type { SSHStatus } from '@/types/setup'

const props = defineProps<{
  ssh: SSHStatus
}>()

const emit = defineEmits<{ saved: [] }>()

const host = ref(props.ssh?.host ?? '')
const sshUser = ref(props.ssh?.user ?? 'root')
const brainPath = ref(props.ssh?.brain_path ?? '/opt/jodo/brain')
const publicKey = ref('')
const generating = ref(false)
const confirmingRegenerate = ref(false)
const verifying = ref(false)
const verifyResult = ref<{ connected: boolean; error?: string } | null>(null)
const error = ref<string | null>(null)

const isDocker = (props.ssh?.jodo_mode ?? 'vps') === 'docker'

function handleRegenerateClick() {
  if (!confirmingRegenerate.value) {
    confirmingRegenerate.value = true
    return
  }
  confirmingRegenerate.value = false
  doGenerateKey()
}

function cancelRegenerate() {
  confirmingRegenerate.value = false
}

async function doGenerateKey() {
  generating.value = true
  error.value = null
  try {
    const res = await api.setupSSHGenerate()
    publicKey.value = res.public_key
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Key generation failed'
  } finally {
    generating.value = false
  }
}

async function verifyConnection() {
  verifying.value = true
  error.value = null
  verifyResult.value = null
  try {
    const res = await api.setupSSHVerify(host.value, sshUser.value)
    verifyResult.value = res
    if (res.connected) {
      emit('saved')
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Verification failed'
  } finally {
    verifying.value = false
  }
}

function copyKey() {
  navigator.clipboard.writeText(publicKey.value)
}
</script>

<template>
  <div class="space-y-4">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <!-- Mode & Brain Path -->
    <Card class="p-4 space-y-4">
      <div class="flex items-center gap-2">
        <span class="text-sm font-medium">Mode:</span>
        <Badge :variant="isDocker ? 'secondary' : 'default'">
          {{ isDocker ? 'Docker' : 'VPS' }}
        </Badge>
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">Brain Path</label>
        <div class="text-sm font-mono text-muted-foreground bg-muted rounded-md px-3 py-2">
          {{ brainPath }}
        </div>
      </div>
    </Card>

    <!-- Connection -->
    <Card class="p-4 space-y-4">
      <div>
        <label class="text-sm font-medium mb-1.5 block">{{ isDocker ? 'Container Host' : 'VPS Host (IP or hostname)' }}</label>
        <Input v-model="host" :placeholder="isDocker ? 'jodo' : '1.2.3.4'" :disabled="isDocker" />
      </div>

      <div>
        <label class="text-sm font-medium mb-1.5 block">SSH User</label>
        <Input v-model="sshUser" placeholder="root" :disabled="isDocker" />
      </div>
    </Card>

    <Card v-if="!isDocker && ssh?.has_key" class="p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-medium">SSH Key</h3>
          <p class="text-xs text-muted-foreground mt-0.5">Key is configured.</p>
        </div>
        <div v-if="confirmingRegenerate" class="flex items-center gap-2">
          <span class="text-xs text-destructive">This will overwrite the existing key.</span>
          <Button @click="handleRegenerateClick" :disabled="generating" variant="destructive" size="sm">
            Confirm
          </Button>
          <Button @click="cancelRegenerate" variant="ghost" size="sm">
            Cancel
          </Button>
        </div>
        <Button v-else @click="handleRegenerateClick" :disabled="generating" variant="outline">
          {{ generating ? 'Generating...' : 'Regenerate Key' }}
        </Button>
      </div>

      <div v-if="publicKey" class="space-y-2">
        <label class="text-sm font-medium block">Public Key</label>
        <div class="relative">
          <pre class="text-xs bg-muted p-3 rounded-md overflow-x-auto font-mono break-all whitespace-pre-wrap">{{ publicKey }}</pre>
          <button
            @click="copyKey"
            class="absolute top-2 right-2 text-xs text-muted-foreground hover:text-foreground px-2 py-1 rounded bg-background border"
          >
            Copy
          </button>
        </div>
        <p class="text-xs text-muted-foreground">
          Add this key to <code class="text-xs">~/.ssh/authorized_keys</code> on the Jodo VPS.
        </p>
      </div>
    </Card>

    <div class="flex items-center gap-3">
      <Button @click="verifyConnection" :disabled="verifying || !host">
        {{ verifying ? 'Verifying...' : 'Verify Connection' }}
      </Button>

      <p v-if="verifyResult?.connected" class="text-sm text-green-500">
        Connected successfully.
      </p>
      <p v-else-if="verifyResult && !verifyResult.connected" class="text-sm text-destructive">
        Connection failed: {{ verifyResult.error || 'Unknown error' }}
      </p>
    </div>
  </div>
</template>

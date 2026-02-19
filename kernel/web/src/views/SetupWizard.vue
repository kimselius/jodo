<script setup lang="ts">
import { useSetup } from '@/composables/useSetup'
import StepVPS from '@/components/setup/StepVPS.vue'
import StepServerSetup from '@/components/setup/StepServerSetup.vue'
import StepKernelURL from '@/components/setup/StepKernelURL.vue'
import StepProviders from '@/components/setup/StepProviders.vue'
import StepGenesis from '@/components/setup/StepGenesis.vue'
import StepReview from '@/components/setup/StepReview.vue'

const setup = useSetup()
</script>

<template>
  <div class="min-h-screen bg-background flex flex-col">
    <!-- Header -->
    <header class="border-b border-border px-6 py-4">
      <div class="max-w-3xl mx-auto flex items-center gap-3">
        <div class="flex h-9 w-9 items-center justify-center rounded-full bg-primary/15">
          <span class="text-primary text-lg font-bold">J</span>
        </div>
        <div>
          <h1 class="text-lg font-semibold">Jodo Setup</h1>
          <p class="text-xs text-muted-foreground">Configure your AI companion</p>
        </div>
      </div>
    </header>

    <!-- Step indicator -->
    <div class="border-b border-border px-6 py-3">
      <div class="max-w-3xl mx-auto flex gap-1">
        <div
          v-for="(step, i) in setup.steps"
          :key="step"
          :class="[
            'h-1 flex-1 rounded-full transition-colors',
            i <= setup.currentStepIndex() ? 'bg-primary' : 'bg-muted'
          ]"
        />
      </div>
    </div>

    <!-- Error banner -->
    <div v-if="setup.error.value" class="px-6 py-2">
      <div class="max-w-3xl mx-auto">
        <p class="text-sm text-destructive bg-destructive/10 rounded-md px-3 py-2">
          {{ setup.error.value }}
        </p>
      </div>
    </div>

    <!-- Content -->
    <main class="flex-1 overflow-y-auto px-6 py-6">
      <div class="max-w-3xl mx-auto">
        <StepVPS
          v-if="setup.currentStep.value === 'vps'"
          :vps="setup.vps"
          :jodo-mode="setup.jodoMode.value"
          :error="setup.error.value"
          @generate="setup.generateSSHKey()"
          @verify="setup.verifySSH()"
          @install-docker-key="setup.installDockerKey()"
          @next="setup.nextStep()"
        />

        <StepServerSetup
          v-else-if="setup.currentStep.value === 'server-setup'"
          :brain-path="setup.brainPath.value"
          :provisioning="setup.provisioning.value"
          :provision-steps="setup.provisionSteps.value"
          :provisioned="setup.provisioned.value"
          :error="setup.error.value"
          @update:brain-path="setup.brainPath.value = $event"
          @provision="setup.provisionServer()"
          @next="setup.nextStep()"
          @back="setup.prevStep()"
        />

        <StepKernelURL
          v-else-if="setup.currentStep.value === 'kernel-url'"
          v-model="setup.kernelUrl.value"
          @next="setup.nextStep()"
          @back="setup.prevStep()"
        />

        <StepProviders
          v-else-if="setup.currentStep.value === 'providers'"
          :providers="setup.providers.value"
          @next="setup.nextStep()"
          @back="setup.prevStep()"
        />

        <StepGenesis
          v-else-if="setup.currentStep.value === 'genesis'"
          :genesis="setup.genesis.value"
          @next="setup.nextStep()"
          @back="setup.prevStep()"
        />

        <StepReview
          v-else-if="setup.currentStep.value === 'review'"
          :vps="setup.vps"
          :kernel-url="setup.kernelUrl.value"
          :brain-path="setup.brainPath.value"
          :jodo-mode="setup.jodoMode.value"
          :providers="setup.providers.value"
          :genesis="setup.genesis.value"
          :birthing="setup.birthing.value"
          @birth="setup.birth()"
          @back="setup.prevStep()"
        />
      </div>
    </main>
  </div>
</template>

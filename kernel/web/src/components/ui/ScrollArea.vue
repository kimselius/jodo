<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string }>()
</script>

<template>
  <div :class="cn('relative overflow-auto', $props.class)">
    <slot />
  </div>
</template>

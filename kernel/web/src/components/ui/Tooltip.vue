<script setup lang="ts">
import { ref } from 'vue'

const show = ref(false)
</script>

<template>
  <div class="relative inline-flex" @mouseenter="show = true" @mouseleave="show = false">
    <slot />
    <div
      v-if="show"
      class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-2 py-1 text-xs rounded bg-popover text-popover-foreground border border-border whitespace-nowrap z-50"
    >
      <slot name="content" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'secondary' | 'success' | 'warning' | 'destructive' | 'accent'
}>(), {
  variant: 'default',
})

const classes = computed(() => cn(
  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
  {
    'bg-primary/15 text-primary': props.variant === 'default',
    'bg-secondary text-muted-foreground': props.variant === 'secondary',
    'bg-success/15 text-success': props.variant === 'success',
    'bg-warning/15 text-warning': props.variant === 'warning',
    'bg-destructive/15 text-destructive': props.variant === 'destructive',
    'bg-accent/15 text-accent': props.variant === 'accent',
  }
))
</script>

<template>
  <span :class="classes">
    <slot />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'secondary' | 'ghost' | 'destructive' | 'outline'
  size?: 'default' | 'sm' | 'lg' | 'icon'
  disabled?: boolean
}>(), {
  variant: 'default',
  size: 'default',
  disabled: false,
})

const classes = computed(() => cn(
  'inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors',
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
  'disabled:pointer-events-none disabled:opacity-50',
  {
    'bg-primary text-primary-foreground hover:bg-primary/90': props.variant === 'default',
    'bg-secondary text-secondary-foreground hover:bg-secondary/80': props.variant === 'secondary',
    'hover:bg-secondary hover:text-foreground': props.variant === 'ghost',
    'bg-destructive text-destructive-foreground hover:bg-destructive/90': props.variant === 'destructive',
    'border border-border bg-transparent hover:bg-secondary': props.variant === 'outline',
  },
  {
    'h-9 px-4 py-2': props.size === 'default',
    'h-8 px-3 text-xs': props.size === 'sm',
    'h-10 px-6': props.size === 'lg',
    'h-9 w-9 p-0': props.size === 'icon',
  }
))
</script>

<template>
  <button :class="classes" :disabled="disabled">
    <slot />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'
import type { GallaEntry } from '@/types/growth'

const props = defineProps<{ galla: GallaEntry }>()

const isComplete = computed(() => props.galla.completed_at !== null)

const ERROR_PATTERNS = [
  "couldn't reach the kernel",
  "tool loop limit",
  "think failed",
  "planning failed",
]

const isErrorSummary = computed(() => {
  const s = (props.galla.summary || '').toLowerCase()
  return ERROR_PATTERNS.some(p => s.includes(p))
})

const hasPlan = computed(() => {
  const p = props.galla.plan
  if (!p) return false
  if (p === '(birth — before galla tracking)') return false
  return true
})

const isPlanError = computed(() => {
  const p = (props.galla.plan || '').toLowerCase()
  return p.startsWith('\u26a0') || ERROR_PATTERNS.some(pat => p.includes(pat))
})

function renderMd(text: string | null): string {
  if (!text) return ''
  return marked.parse(text, { async: false }) as string
}
</script>

<template>
  <Card class="p-4 space-y-3">
    <!-- Header -->
    <div class="flex items-center gap-2 flex-wrap">
      <span class="text-sm font-semibold">Galla {{ galla.galla }}</span>
      <Badge v-if="isErrorSummary" variant="destructive" class="text-[10px]">error</Badge>
      <Badge v-else-if="isComplete" variant="success" class="text-[10px]">
        {{ galla.actions_count }} action{{ galla.actions_count !== 1 ? 's' : '' }}
      </Badge>
      <Badge v-else variant="warning" class="text-[10px]">in progress</Badge>
      <span class="text-[10px] text-muted-foreground ml-auto">{{ formatTime(galla.started_at) }}</span>
    </div>

    <!-- Plan -->
    <div v-if="hasPlan">
      <span class="text-xs font-medium text-muted-foreground" :class="isPlanError ? 'text-destructive/70' : ''">
        {{ isPlanError ? 'Plan (failed)' : 'Plan' }}
      </span>
      <div
        :class="[
          'mt-1.5 pl-3 border-l-2 prose prose-sm prose-invert max-w-none text-xs [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded-md [&_pre]:text-xs [&_code]:text-xs [&_code]:bg-muted [&_code]:px-1 [&_code]:rounded [&_p]:my-1 [&_ul]:my-1 [&_ol]:my-1 [&_li]:my-0.5',
          isPlanError ? 'border-destructive/30 text-muted-foreground italic' : 'border-border text-muted-foreground'
        ]"
        v-html="renderMd(galla.plan)"
      />
    </div>

    <!-- In progress spinner -->
    <div v-if="!isComplete && !galla.summary" class="flex items-center gap-2 py-2">
      <div class="h-4 w-4 rounded-full border-2 border-primary/30 border-t-primary animate-spin" />
      <span class="text-xs text-muted-foreground">Executing...</span>
    </div>

    <!-- Error summary -->
    <div v-if="isErrorSummary" class="text-sm text-muted-foreground italic">
      {{ galla.summary }}
    </div>

    <!-- Summary -->
    <template v-else-if="galla.summary">
      <span class="text-xs font-medium text-muted-foreground">Summary</span>
      <div
        class="prose prose-sm prose-invert max-w-none text-sm text-foreground [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:rounded-md [&_pre]:text-xs [&_code]:text-xs [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_p]:my-1.5 [&_ul]:my-1.5 [&_ol]:my-1.5 [&_li]:my-0.5 [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-sm [&_h1]:font-semibold [&_h2]:font-semibold [&_h3]:font-medium"
        v-html="renderMd(galla.summary)"
      />
    </template>
  </Card>
</template>

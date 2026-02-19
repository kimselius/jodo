<script setup lang="ts">
import { ref, computed } from 'vue'
import { marked } from 'marked'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import { formatTime } from '@/lib/utils'
import type { GallaEntry } from '@/types/growth'

const props = defineProps<{ galla: GallaEntry }>()

const planOpen = ref(false)
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
  // Hide placeholder plans
  if (p === '(birth — no planning phase)' || p === '(birth — before galla tracking)') return false
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

    <!-- Error summary -->
    <div v-if="isErrorSummary" class="text-sm text-muted-foreground italic">
      {{ galla.summary }}
    </div>

    <!-- Summary (main content) -->
    <div
      v-else-if="galla.summary"
      class="prose prose-sm prose-invert max-w-none text-sm text-foreground [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:rounded-md [&_pre]:text-xs [&_code]:text-xs [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_p]:my-1.5 [&_ul]:my-1.5 [&_ol]:my-1.5 [&_li]:my-0.5 [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-sm [&_h1]:font-semibold [&_h2]:font-semibold [&_h3]:font-medium"
      v-html="renderMd(galla.summary)"
    />

    <!-- In progress spinner -->
    <div v-else-if="!isComplete" class="flex items-center gap-2 py-2">
      <div class="h-4 w-4 rounded-full border-2 border-primary/30 border-t-primary animate-spin" />
      <span class="text-xs text-muted-foreground">Thinking...</span>
    </div>

    <!-- Plan (collapsible) -->
    <div v-if="hasPlan">
      <button
        @click="planOpen = !planOpen"
        class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        <svg
          class="h-3 w-3 transition-transform"
          :class="planOpen ? 'rotate-90' : ''"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
        </svg>
        <span :class="isPlanError ? 'text-destructive/70' : ''">
          {{ isPlanError ? 'Plan (failed)' : 'Plan' }}
        </span>
      </button>
      <div
        v-if="planOpen"
        :class="[
          'mt-2 pl-3 border-l-2 prose prose-sm prose-invert max-w-none text-xs [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded-md [&_pre]:text-xs [&_code]:text-xs [&_code]:bg-muted [&_code]:px-1 [&_code]:rounded [&_p]:my-1 [&_ul]:my-1 [&_ol]:my-1 [&_li]:my-0.5',
          isPlanError ? 'border-destructive/30 text-muted-foreground italic' : 'border-border text-muted-foreground'
        ]"
        v-html="renderMd(galla.plan)"
      />
    </div>
  </Card>
</template>

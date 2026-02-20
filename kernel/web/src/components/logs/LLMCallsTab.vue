<script setup lang="ts">
import { ref } from 'vue'
import { useLLMCalls } from '@/composables/useLLMCalls'
import type { JodoMessage } from '@/types/llmcalls'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'

const {
  calls, total, totalTokensIn, totalTokensOut, totalCost,
  loading, error,
  load, loadMore, hasMore,
  selectedCall, detailLoading, toggleDetail,
} = useLLMCalls()

// Collapsible sections within the expanded detail
const openSections = ref(new Set<string>())

function toggleSection(key: string) {
  if (openSections.value.has(key)) {
    openSections.value.delete(key)
  } else {
    openSections.value.add(key)
  }
}

function isSectionOpen(key: string): boolean {
  return openSections.value.has(key)
}

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function fmtCost(c: number): string {
  if (c === 0) return '-'
  if (c < 0.001) return '$' + c.toFixed(6)
  return '$' + c.toFixed(4)
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

function fmtTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function fmtModel(provider: string, model: string): string {
  const short = model.length > 24 ? model.slice(0, 22) + '..' : model
  return `${provider}/${short}`
}

function extractLastUserMessage(messages: JodoMessage[]): string | null {
  if (!messages?.length) return null
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === 'user' && messages[i].content) {
      return messages[i].content
    }
  }
  return null
}

function tryFormatJSON(raw: unknown): string {
  if (raw == null) return ''
  if (typeof raw === 'string') return raw
  try {
    return JSON.stringify(raw, null, 2)
  } catch {
    return String(raw)
  }
}

const roleBadgeClass: Record<string, string> = {
  user: 'bg-blue-500/15 text-blue-400',
  assistant: 'bg-emerald-500/15 text-emerald-400',
  tool: 'bg-amber-500/15 text-amber-400',
  system: 'bg-purple-500/15 text-purple-400',
}

function handleToggle(id: number) {
  openSections.value.clear()
  toggleDetail(id)
}
</script>

<template>
  <div>
    <!-- Summary header -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-4 text-xs text-muted-foreground">
        <span>{{ total }} calls</span>
        <span>{{ fmtTokens(totalTokensIn) }} in / {{ fmtTokens(totalTokensOut) }} out</span>
        <span>{{ fmtCost(totalCost) }}</span>
      </div>
      <Button variant="ghost" size="sm" @click="load(true)">Refresh</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

    <!-- Table -->
    <div v-if="loading && calls.length === 0" class="flex items-center justify-center py-12">
      <span class="text-sm text-muted-foreground">Loading...</span>
    </div>

    <div v-else-if="calls.length === 0" class="text-center py-12">
      <p class="text-sm text-muted-foreground">No LLM calls recorded yet.</p>
    </div>

    <div v-else>
      <!-- Table header -->
      <div class="grid grid-cols-[auto_1fr_auto_auto_auto_auto_auto] gap-2 px-2 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider border-b border-border">
        <span class="w-10 text-right">#</span>
        <span>Intent / Model</span>
        <span class="text-right w-14">In</span>
        <span class="text-right w-14">Out</span>
        <span class="text-right w-16">Cost</span>
        <span class="text-right w-14">Time</span>
        <span class="text-right w-28">When</span>
      </div>

      <!-- Rows with inline expansion -->
      <template v-for="call in calls" :key="call.id">
        <!-- Summary row -->
        <div
          @click="handleToggle(call.id)"
          :class="[
            'grid grid-cols-[auto_1fr_auto_auto_auto_auto_auto] gap-2 px-2 py-2 text-sm cursor-pointer transition-colors border-b border-border/50',
            selectedCall?.id === call.id
              ? 'bg-primary/5 border-b-0'
              : 'hover:bg-secondary/30',
            call.error ? 'text-destructive/80' : 'text-foreground'
          ]"
        >
          <span class="w-10 text-right text-xs text-muted-foreground tabular-nums">{{ call.id }}</span>
          <div class="min-w-0">
            <div class="flex items-center gap-1.5">
              <Badge :variant="call.error ? 'destructive' : 'secondary'" class="text-[10px] shrink-0">{{ call.intent }}</Badge>
              <span class="text-xs text-muted-foreground truncate">{{ fmtModel(call.provider, call.model) }}</span>
            </div>
          </div>
          <span class="text-xs text-muted-foreground text-right w-14 tabular-nums">{{ fmtTokens(call.tokens_in) }}</span>
          <span class="text-xs text-muted-foreground text-right w-14 tabular-nums">{{ fmtTokens(call.tokens_out) }}</span>
          <span class="text-xs text-muted-foreground text-right w-16 tabular-nums">{{ fmtCost(call.cost) }}</span>
          <span class="text-xs text-muted-foreground text-right w-14 tabular-nums">{{ fmtDuration(call.duration_ms) }}</span>
          <span class="text-xs text-muted-foreground text-right w-28">{{ fmtTime(call.created_at) }}</span>
        </div>

        <!-- Inline detail (expands below the row) -->
        <div v-if="selectedCall?.id === call.id" class="border-b border-border bg-card/50 px-4 py-3">
          <div v-if="detailLoading" class="flex items-center justify-center py-6">
            <span class="text-sm text-muted-foreground">Loading...</span>
          </div>

          <div v-else class="space-y-3">
            <!-- Stats -->
            <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span>{{ fmtTokens(selectedCall.tokens_in) }} in / {{ fmtTokens(selectedCall.tokens_out) }} out</span>
              <span>{{ fmtCost(selectedCall.cost) }}</span>
              <span>{{ fmtDuration(selectedCall.duration_ms) }}</span>
              <span>{{ fmtTime(selectedCall.created_at) }}</span>
            </div>

            <!-- Error -->
            <div v-if="selectedCall.error" class="rounded-md bg-destructive/10 p-3">
              <pre class="text-xs text-destructive whitespace-pre-wrap">{{ selectedCall.error }}</pre>
            </div>

            <!-- System Prompt (collapsible, closed by default) -->
            <div v-if="selectedCall.request_system">
              <button
                @click="toggleSection('system')"
                class="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                <span class="text-[10px]">{{ isSectionOpen('system') ? '▾' : '▸' }}</span>
                System Prompt
              </button>
              <pre
                v-if="isSectionOpen('system')"
                class="mt-1 text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-48 overflow-y-auto"
              >{{ selectedCall.request_system }}</pre>
            </div>

            <!-- Request (last user message) -->
            <div v-if="extractLastUserMessage(selectedCall.request_messages)">
              <p class="text-xs font-medium text-muted-foreground mb-1">Request</p>
              <pre class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-60 overflow-y-auto">{{ extractLastUserMessage(selectedCall.request_messages) }}</pre>
            </div>

            <!-- Response: text content and/or tool calls -->
            <div v-if="selectedCall.response_content || selectedCall.response_tool_calls?.length">
              <p class="text-xs font-medium text-muted-foreground mb-1">Response</p>
              <!-- Text content -->
              <pre
                v-if="selectedCall.response_content"
                class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-60 overflow-y-auto"
                :class="selectedCall.response_tool_calls?.length ? 'mb-2' : ''"
              >{{ selectedCall.response_content }}</pre>
              <!-- Tool calls (shown inline as part of the response, not hidden) -->
              <div v-if="selectedCall.response_tool_calls?.length" class="space-y-1.5">
                <div
                  v-for="(tc, i) in selectedCall.response_tool_calls"
                  :key="i"
                  class="rounded-md bg-secondary/30 p-2.5"
                >
                  <span class="inline-block text-[10px] font-medium px-1.5 py-0.5 rounded mb-1 bg-amber-500/15 text-amber-400">
                    {{ (tc as any).name || 'tool' }}
                  </span>
                  <pre class="text-xs whitespace-pre-wrap break-words max-h-40 overflow-y-auto">{{ tryFormatJSON((tc as any).arguments || (tc as any).input || tc) }}</pre>
                </div>
              </div>
            </div>
            <div v-else-if="!selectedCall.error">
              <p class="text-xs text-muted-foreground italic">No response content</p>
            </div>

            <!-- Request Tools (collapsible, if present) -->
            <div v-if="selectedCall.request_tools?.length">
              <button
                @click="toggleSection('req-tools')"
                class="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                <span class="text-[10px]">{{ isSectionOpen('req-tools') ? '▾' : '▸' }}</span>
                Available Tools ({{ selectedCall.request_tools.length }})
              </button>
              <pre
                v-if="isSectionOpen('req-tools')"
                class="mt-1 text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-48 overflow-y-auto"
              >{{ tryFormatJSON(selectedCall.request_tools) }}</pre>
            </div>

            <!-- Full Context (collapsible, for debugging) -->
            <div v-if="selectedCall.request_messages?.length > 1">
              <button
                @click="toggleSection('context')"
                class="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                <span class="text-[10px]">{{ isSectionOpen('context') ? '▾' : '▸' }}</span>
                Full Context ({{ selectedCall.request_messages.length }} messages)
              </button>
              <div v-if="isSectionOpen('context')" class="mt-1 space-y-1.5">
                <div
                  v-for="(msg, i) in selectedCall.request_messages"
                  :key="i"
                  class="rounded-md bg-secondary/30 p-2.5"
                >
                  <span
                    :class="[
                      'inline-block text-[10px] font-medium px-1.5 py-0.5 rounded mb-1',
                      roleBadgeClass[msg.role] || 'bg-secondary text-muted-foreground'
                    ]"
                  >{{ msg.role }}</span>
                  <pre class="text-xs whitespace-pre-wrap break-words max-h-32 overflow-y-auto">{{ msg.content || tryFormatJSON(msg.tool_calls) }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Load more -->
      <div v-if="hasMore()" class="flex justify-center pt-3 pb-4">
        <Button variant="ghost" size="sm" :disabled="loading" @click="loadMore">
          {{ loading ? 'Loading...' : 'Load more' }}
        </Button>
      </div>
    </div>
  </div>
</template>

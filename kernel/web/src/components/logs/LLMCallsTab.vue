<script setup lang="ts">
import { useLLMCalls } from '@/composables/useLLMCalls'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'

const {
  calls, total, loading, error,
  load, loadMore, hasMore,
  selectedCall, detailLoading, loadDetail, clearDetail,
} = useLLMCalls()

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
  // Shorten long model names
  const short = model.length > 24 ? model.slice(0, 22) + '..' : model
  return `${provider}/${short}`
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
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-3">
      <p class="text-xs text-muted-foreground">{{ total }} calls recorded</p>
      <Button variant="ghost" size="sm" @click="load(true)">Refresh</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

    <!-- Detail overlay -->
    <div v-if="selectedCall" class="mb-4 rounded-lg border border-border bg-card overflow-hidden">
      <div class="flex items-center justify-between px-4 py-2 bg-secondary/30 border-b border-border">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium">Call #{{ selectedCall.id }}</span>
          <Badge variant="secondary" class="text-[10px]">{{ selectedCall.intent }}</Badge>
          <span class="text-xs text-muted-foreground">{{ selectedCall.provider }}/{{ selectedCall.model }}</span>
        </div>
        <button @click="clearDetail" class="text-muted-foreground hover:text-foreground text-sm px-2">Close</button>
      </div>

      <div v-if="detailLoading" class="flex items-center justify-center py-8">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
        <!-- Stats row -->
        <div class="flex gap-4 text-xs text-muted-foreground">
          <span>Tokens: {{ fmtTokens(selectedCall.tokens_in) }} in / {{ fmtTokens(selectedCall.tokens_out) }} out</span>
          <span>Cost: {{ fmtCost(selectedCall.cost) }}</span>
          <span>Duration: {{ fmtDuration(selectedCall.duration_ms) }}</span>
          <span>{{ fmtTime(selectedCall.created_at) }}</span>
        </div>

        <!-- Error -->
        <div v-if="selectedCall.error" class="rounded-md bg-destructive/10 p-3">
          <p class="text-xs font-medium text-destructive mb-1">Error</p>
          <pre class="text-xs text-destructive whitespace-pre-wrap">{{ selectedCall.error }}</pre>
        </div>

        <!-- System prompt -->
        <div v-if="selectedCall.request_system">
          <p class="text-xs font-medium text-muted-foreground mb-1">System Prompt</p>
          <pre class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-40 overflow-y-auto">{{ selectedCall.request_system }}</pre>
        </div>

        <!-- Messages -->
        <div v-if="selectedCall.request_messages?.length">
          <p class="text-xs font-medium text-muted-foreground mb-1">Messages ({{ selectedCall.request_messages.length }})</p>
          <div class="space-y-2">
            <div
              v-for="(msg, i) in selectedCall.request_messages"
              :key="i"
              class="rounded-md bg-secondary/30 p-3"
            >
              <pre class="text-xs whitespace-pre-wrap break-words max-h-40 overflow-y-auto">{{ tryFormatJSON(msg) }}</pre>
            </div>
          </div>
        </div>

        <!-- Response content -->
        <div v-if="selectedCall.response_content">
          <p class="text-xs font-medium text-muted-foreground mb-1">Response</p>
          <pre class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-60 overflow-y-auto">{{ selectedCall.response_content }}</pre>
        </div>

        <!-- Tool calls -->
        <div v-if="selectedCall.response_tool_calls?.length">
          <p class="text-xs font-medium text-muted-foreground mb-1">Tool Calls ({{ selectedCall.response_tool_calls.length }})</p>
          <pre class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-40 overflow-y-auto">{{ tryFormatJSON(selectedCall.response_tool_calls) }}</pre>
        </div>

        <!-- Request tools -->
        <div v-if="selectedCall.request_tools?.length">
          <p class="text-xs font-medium text-muted-foreground mb-1">Available Tools ({{ selectedCall.request_tools.length }})</p>
          <pre class="text-xs bg-secondary/30 rounded-md p-3 whitespace-pre-wrap break-words max-h-32 overflow-y-auto">{{ tryFormatJSON(selectedCall.request_tools) }}</pre>
        </div>
      </div>
    </div>

    <!-- Table -->
    <div v-if="loading && calls.length === 0" class="flex items-center justify-center py-12">
      <span class="text-sm text-muted-foreground">Loading...</span>
    </div>

    <div v-else-if="calls.length === 0" class="text-center py-12">
      <p class="text-sm text-muted-foreground">No LLM calls recorded yet.</p>
    </div>

    <div v-else>
      <!-- Table header -->
      <div class="grid grid-cols-[1fr_auto_auto_auto_auto] gap-2 px-2 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider border-b border-border">
        <span>Intent / Model</span>
        <span class="text-right w-16">Tokens</span>
        <span class="text-right w-16">Cost</span>
        <span class="text-right w-14">Time</span>
        <span class="text-right w-28">When</span>
      </div>

      <!-- Rows -->
      <div
        v-for="call in calls"
        :key="call.id"
        @click="loadDetail(call.id)"
        :class="[
          'grid grid-cols-[1fr_auto_auto_auto_auto] gap-2 px-2 py-2 text-sm cursor-pointer transition-colors border-b border-border/50',
          selectedCall?.id === call.id
            ? 'bg-primary/5'
            : 'hover:bg-secondary/30',
          call.error ? 'text-destructive/80' : 'text-foreground'
        ]"
      >
        <div class="min-w-0">
          <div class="flex items-center gap-1.5">
            <Badge :variant="call.error ? 'destructive' : 'secondary'" class="text-[10px] shrink-0">{{ call.intent }}</Badge>
            <span class="text-xs text-muted-foreground truncate">{{ fmtModel(call.provider, call.model) }}</span>
          </div>
        </div>
        <span class="text-xs text-muted-foreground text-right w-16 tabular-nums">{{ fmtTokens(call.tokens_in + call.tokens_out) }}</span>
        <span class="text-xs text-muted-foreground text-right w-16 tabular-nums">{{ fmtCost(call.cost) }}</span>
        <span class="text-xs text-muted-foreground text-right w-14 tabular-nums">{{ fmtDuration(call.duration_ms) }}</span>
        <span class="text-xs text-muted-foreground text-right w-28">{{ fmtTime(call.created_at) }}</span>
      </div>

      <!-- Load more -->
      <div v-if="hasMore()" class="flex justify-center pt-3 pb-4">
        <Button variant="ghost" size="sm" :disabled="loading" @click="loadMore">
          {{ loading ? 'Loading...' : 'Load more' }}
        </Button>
      </div>
    </div>
  </div>
</template>

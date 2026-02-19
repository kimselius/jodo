<script setup lang="ts">
import { useChat } from '@/composables/useChat'
import ChatMessageList from '@/components/chat/ChatMessageList.vue'
import ChatInput from '@/components/chat/ChatInput.vue'

const { messages, loading, sending, connected, send } = useChat()
</script>

<template>
  <div class="flex flex-1 flex-col h-full min-h-0">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-border px-4 py-2.5">
      <h1 class="text-sm font-semibold">Chat</h1>
      <div class="flex items-center gap-1.5">
        <span
          :class="['h-2 w-2 rounded-full', connected ? 'bg-success' : 'bg-destructive']"
        />
        <span class="text-xs text-muted-foreground">
          {{ connected ? 'Live' : 'Reconnecting...' }}
        </span>
      </div>
    </div>

    <!-- Messages -->
    <ChatMessageList :messages="messages" :loading="loading" />

    <!-- Input -->
    <ChatInput :sending="sending" @send="send" />
  </div>
</template>

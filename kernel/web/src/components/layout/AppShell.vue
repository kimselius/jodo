<script setup lang="ts">
import { ref } from 'vue'
import Sidebar from './Sidebar.vue'

const mobileOpen = ref(false)
</script>

<template>
  <div class="flex h-screen overflow-hidden">
    <!-- Desktop sidebar -->
    <aside class="hidden md:flex w-56 flex-shrink-0 border-r border-border">
      <Sidebar class="w-full" />
    </aside>

    <!-- Mobile sidebar overlay -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="mobileOpen"
          class="fixed inset-0 z-40 bg-black/50 md:hidden"
          @click="mobileOpen = false"
        />
      </Transition>
      <Transition name="slide">
        <aside
          v-if="mobileOpen"
          class="fixed inset-y-0 left-0 z-50 w-64 md:hidden"
        >
          <Sidebar @close="mobileOpen = false" />
        </aside>
      </Transition>
    </Teleport>

    <!-- Main content -->
    <main class="flex flex-1 flex-col min-w-0">
      <!-- Mobile header -->
      <header class="flex md:hidden items-center gap-3 border-b border-border px-4 py-3">
        <button
          @click="mobileOpen = true"
          class="text-muted-foreground hover:text-foreground"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span class="text-sm font-semibold">Jodo</span>
      </header>

      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.2s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(-100%);
}
</style>

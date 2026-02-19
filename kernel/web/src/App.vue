<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/components/layout/AppShell.vue'

const route = useRoute()
const isSetupRoute = computed(() => route.meta.setupRoute === true)
</script>

<template>
  <!-- Setup wizard has its own full-page layout -->
  <RouterView v-if="isSetupRoute" />
  <!-- Normal app with sidebar -->
  <AppShell v-else />
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useLibrary } from '@/composables/useLibrary'
import LibraryCard from '@/components/library/LibraryCard.vue'
import LibraryForm from '@/components/library/LibraryForm.vue'
import CommentThread from '@/components/library/CommentThread.vue'
import Button from '@/components/ui/Button.vue'

const { items, loading, error, load, create, update, remove, comment, patchStatus } = useLibrary()

const showForm = ref(false)
const editingId = ref<number | null>(null)
const expandedId = ref<number | null>(null)
const activeFilter = ref<string | null>(null)

const filters = [
  { key: null, label: 'All' },
  { key: 'new', label: 'New' },
  { key: 'in_progress', label: 'In Progress' },
  { key: 'done', label: 'Done' },
  { key: 'blocked', label: 'Blocked' },
]

const filteredItems = computed(() => {
  if (!activeFilter.value) return items.value
  return items.value.filter(i => i.status === activeFilter.value)
})

const activeCount = computed(() =>
  items.value.filter(i => i.status !== 'done' && i.status !== 'archived').length
)

function toggleExpand(id: number) {
  expandedId.value = expandedId.value === id ? null : id
  editingId.value = null
}

async function handleCreate(data: { title: string; content: string; priority: number }) {
  await create(data.title, data.content, data.priority)
  showForm.value = false
}

async function handleUpdate(id: number, data: { title: string; content: string; priority: number }) {
  await update(id, data)
  editingId.value = null
}

function startEdit(id: number) {
  editingId.value = id
  expandedId.value = id
}
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div class="max-w-2xl mx-auto p-4">
      <!-- Header -->
      <div class="flex items-center justify-between mb-4">
        <div>
          <h1 class="text-lg font-semibold">Library</h1>
          <p class="text-xs text-muted-foreground mt-0.5">{{ activeCount }} active briefs</p>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="load">Refresh</Button>
          <Button size="sm" @click="showForm = !showForm">
            {{ showForm ? 'Cancel' : 'New Brief' }}
          </Button>
        </div>
      </div>

      <!-- Create form -->
      <div v-if="showForm" class="mb-4">
        <LibraryForm @submit="handleCreate" @cancel="showForm = false" />
      </div>

      <!-- Status filter tabs -->
      <div class="flex items-center gap-1 mb-4 overflow-x-auto">
        <button
          v-for="f in filters"
          :key="f.key ?? 'all'"
          @click="activeFilter = f.key"
          class="rounded-md px-3 py-1 text-xs font-medium transition-colors shrink-0"
          :class="activeFilter === f.key
            ? 'bg-primary text-primary-foreground'
            : 'bg-secondary text-muted-foreground hover:text-foreground'"
        >
          {{ f.label }}
        </button>
      </div>

      <p v-if="error" class="text-sm text-destructive mb-4">{{ error }}</p>

      <div v-if="loading && items.length === 0" class="flex items-center justify-center py-12">
        <span class="text-sm text-muted-foreground">Loading...</span>
      </div>

      <div v-else-if="filteredItems.length === 0" class="text-center py-12">
        <p class="text-sm text-muted-foreground">
          {{ activeFilter ? 'No items with this status.' : 'No briefs yet. Create one to give Jodo a task.' }}
        </p>
      </div>

      <div v-else class="space-y-2">
        <div v-for="item in filteredItems" :key="item.id">
          <!-- Edit mode -->
          <LibraryForm
            v-if="editingId === item.id"
            :initial-title="item.title"
            :initial-content="item.content"
            :initial-priority="item.priority"
            submit-label="Save"
            @submit="(data) => handleUpdate(item.id, data)"
            @cancel="editingId = null"
          />

          <!-- Card mode -->
          <LibraryCard
            v-else
            :item="item"
            :expanded="expandedId === item.id"
            @toggle="toggleExpand(item.id)"
            @update-status="(s) => patchStatus(item.id, s)"
          >
            <template #comments>
              <CommentThread
                :comments="item.comments"
                @comment="(msg) => comment(item.id, msg)"
              />

              <!-- Card actions -->
              <div class="flex items-center gap-2 mt-3 pt-2 border-t border-border">
                <Button variant="ghost" size="sm" @click.stop="startEdit(item.id)">Edit</Button>
                <Button variant="ghost" size="sm" class="text-destructive hover:text-destructive" @click.stop="remove(item.id)">Delete</Button>
              </div>
            </template>
          </LibraryCard>
        </div>
      </div>
    </div>
  </div>
</template>

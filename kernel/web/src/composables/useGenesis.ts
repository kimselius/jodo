import { ref, onMounted } from 'vue'
import { api } from '@/lib/api'
import type { Genesis, IdentityUpdate } from '@/types/genesis'

export function useGenesis() {
  const genesis = ref<Genesis | null>(null)
  const loading = ref(true)
  const saving = ref(false)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    try {
      genesis.value = await api.getGenesis()
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load genesis'
    } finally {
      loading.value = false
    }
  }

  async function updateIdentity(update: IdentityUpdate): Promise<boolean> {
    saving.value = true
    try {
      await api.updateIdentity(update)
      // Refresh to get updated data
      await load()
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save'
      return false
    } finally {
      saving.value = false
    }
  }

  onMounted(load)

  return { genesis, loading, saving, error, load, updateIdentity }
}

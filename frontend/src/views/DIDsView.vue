<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Phone, Plus, Edit2, Trash2, RefreshCw, ExternalLink } from 'lucide-vue-next'
import api from '@/api/client'

interface DID {
  id: number
  phone_number: string
  friendly_name: string
  twilio_sid: string
  capabilities: {
    voice: boolean
    sms: boolean
    mms: boolean
  }
  voice_url?: string
  sms_url?: string
  created_at: string
}

const dids = ref<DID[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const showModal = ref(false)
const editingDID = ref<DID | null>(null)
const saving = ref(false)
const syncingFromTwilio = ref(false)

const form = ref({
  phone_number: '',
  friendly_name: ''
})

onMounted(async () => {
  await loadDIDs()
})

async function loadDIDs() {
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/dids')
    dids.value = response.data.data || []
  } catch {
    error.value = 'Failed to load phone numbers'
  } finally {
    loading.value = false
  }
}

async function syncFromTwilio() {
  syncingFromTwilio.value = true
  error.value = null
  try {
    await api.post('/dids/sync')
    await loadDIDs()
  } catch {
    error.value = 'Failed to sync from Twilio'
  } finally {
    syncingFromTwilio.value = false
  }
}

function openEditModal(did: DID) {
  editingDID.value = did
  form.value = {
    phone_number: did.phone_number,
    friendly_name: did.friendly_name
  }
  showModal.value = true
}

async function handleSubmit() {
  if (!editingDID.value) return

  saving.value = true
  error.value = null

  try {
    await api.put(`/dids/${editingDID.value.id}`, {
      friendly_name: form.value.friendly_name
    })
    showModal.value = false
    await loadDIDs()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Update failed'
  } finally {
    saving.value = false
  }
}

async function handleDelete(did: DID) {
  if (!confirm(`Remove "${did.friendly_name || did.phone_number}" from GoSIP?\n\nThis will NOT release the number from your Twilio account.`)) return

  try {
    await api.delete(`/dids/${did.id}`)
    await loadDIDs()
  } catch {
    error.value = 'Failed to remove phone number'
  }
}

function formatPhoneNumber(phone: string): string {
  if (phone.startsWith('+1') && phone.length === 12) {
    return `(${phone.slice(2, 5)}) ${phone.slice(5, 8)}-${phone.slice(8)}`
  }
  return phone
}
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Phone Numbers</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ dids.length }} phone number{{ dids.length !== 1 ? 's' : '' }} configured
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadDIDs"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
        <button
          @click="syncFromTwilio"
          :disabled="syncingFromTwilio"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
        >
          <Plus class="h-4 w-4 mr-2" />
          {{ syncingFromTwilio ? 'Syncing...' : 'Sync from Twilio' }}
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else class="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <div
        v-for="did in dids"
        :key="did.id"
        class="bg-white dark:bg-gray-800 shadow rounded-lg p-6"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center">
            <div class="p-2 rounded-full bg-primary/10">
              <Phone class="h-5 w-5 text-primary" />
            </div>
            <div class="ml-3">
              <p class="font-medium text-gray-900 dark:text-white">
                {{ formatPhoneNumber(did.phone_number) }}
              </p>
              <p v-if="did.friendly_name" class="text-sm text-gray-500">
                {{ did.friendly_name }}
              </p>
            </div>
          </div>
          <div class="flex space-x-1">
            <button
              @click="openEditModal(did)"
              class="p-1 text-gray-400 hover:text-primary"
            >
              <Edit2 class="h-4 w-4" />
            </button>
            <button
              @click="handleDelete(did)"
              class="p-1 text-gray-400 hover:text-destructive"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-2">
          <span
            v-if="did.capabilities?.voice"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
          >
            Voice
          </span>
          <span
            v-if="did.capabilities?.sms"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
          >
            SMS
          </span>
          <span
            v-if="did.capabilities?.mms"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
          >
            MMS
          </span>
        </div>

        <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <a
            :href="`https://console.twilio.com/us1/develop/phone-numbers/manage/incoming/${did.twilio_sid}`"
            target="_blank"
            class="text-sm text-primary hover:text-primary/80 flex items-center"
          >
            View in Twilio
            <ExternalLink class="h-3 w-3 ml-1" />
          </a>
        </div>
      </div>

      <div v-if="dids.length === 0" class="col-span-full text-center py-12 text-gray-500">
        No phone numbers configured. Click "Sync from Twilio" to import your numbers.
      </div>
    </div>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Edit Phone Number
          </h3>

          <form @submit.prevent="handleSubmit" class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Phone Number
              </label>
              <input
                :value="formatPhoneNumber(form.phone_number)"
                type="text"
                disabled
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-gray-50 dark:bg-gray-600 text-gray-500 dark:text-gray-400"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Friendly Name
              </label>
              <input
                v-model="form.friendly_name"
                type="text"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="Main Line"
              />
            </div>

            <div class="flex justify-end space-x-3 pt-4">
              <button
                type="button"
                @click="showModal = false"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="saving"
                class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                {{ saving ? 'Saving...' : 'Update' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

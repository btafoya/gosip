<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Server, Plus, Edit2, Trash2, RefreshCw, Shield, Phone } from 'lucide-vue-next'
import api from '@/api/client'
import { trunksApi, type Trunk } from '@/api/trunks'

interface DID {
  id: number
  phone_number: string
  friendly_name: string
}

const trunks = ref<Trunk[]>([])
const dids = ref<DID[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const showModal = ref(false)
const showAssignModal = ref(false)
const editingTrunk = ref<Trunk | null>(null)
const assigningTrunk = ref<Trunk | null>(null)
const saving = ref(false)
const syncing = ref(false)
const assigning = ref(false)

function extractError(err: unknown): string {
  const e = err as { response?: { data?: { error?: { message?: string } } }; message?: string }
  return e.response?.data?.error?.message || e.message || 'Request failed'
}

const form = ref({
  friendly_name: '',
  secure: true,
  transfer_mode: 'disable-all',
  cnam_lookup_enabled: false
})

const transferModes = [
  { value: 'disable-all', label: 'Disable All' },
  { value: 'enable-all', label: 'Enable All' },
  { value: 'sip-only', label: 'SIP Only' }
]

onMounted(async () => {
  await loadTrunks()
  await loadDIDs()
})

async function loadTrunks() {
  loading.value = true
  error.value = null
  try {
    const response = await trunksApi.list()
    trunks.value = response.data || []
  } catch (err: unknown) {
    error.value = extractError(err)
  } finally {
    loading.value = false
  }
}

async function loadDIDs() {
  try {
    const response = await api.get('/dids')
    dids.value = response.data.data || []
  } catch {
    // non-critical
  }
}

async function syncFromTwilio() {
  syncing.value = true
  error.value = null
  try {
    await trunksApi.sync()
    await loadTrunks()
  } catch (err: unknown) {
    error.value = extractError(err)
  } finally {
    syncing.value = false
  }
}

function openCreateModal() {
  editingTrunk.value = null
  form.value = {
    friendly_name: '',
    secure: true,
    transfer_mode: 'disable-all',
    cnam_lookup_enabled: false
  }
  showModal.value = true
}

function openEditModal(trunk: Trunk) {
  editingTrunk.value = trunk
  form.value = {
    friendly_name: trunk.friendly_name,
    secure: trunk.secure,
    transfer_mode: trunk.transfer_mode,
    cnam_lookup_enabled: trunk.cnam_lookup_enabled
  }
  showModal.value = true
}

async function handleSubmit() {
  saving.value = true
  error.value = null

  try {
    if (editingTrunk.value) {
      await trunksApi.update(editingTrunk.value.id, {
        friendly_name: form.value.friendly_name,
        secure: form.value.secure,
        transfer_mode: form.value.transfer_mode,
        cnam_lookup_enabled: form.value.cnam_lookup_enabled
      })
    } else {
      await trunksApi.create({
        friendly_name: form.value.friendly_name,
        secure: form.value.secure,
        transfer_mode: form.value.transfer_mode,
        cnam_lookup_enabled: form.value.cnam_lookup_enabled
      })
    }
    showModal.value = false
    await loadTrunks()
  } catch (err: unknown) {
    error.value = extractError(err)
  } finally {
    saving.value = false
  }
}

async function handleDelete(trunk: Trunk) {
  if (!confirm(`Delete "${trunk.friendly_name || trunk.twilio_sid}"?\n\nThis will also delete the trunk from Twilio.`)) return

  try {
    await trunksApi.delete(trunk.id)
    await loadTrunks()
  } catch (err: unknown) {
    error.value = extractError(err)
  }
}

function openAssignModal(trunk: Trunk) {
  assigningTrunk.value = trunk
  showAssignModal.value = true
}

async function handleAssignDID(didID: number) {
  if (!assigningTrunk.value) return
  assigning.value = true
  error.value = null
  try {
    await trunksApi.assignDID(assigningTrunk.value.id, { did_id: didID })
    showAssignModal.value = false
    await loadDIDs()
  } catch (err: unknown) {
    error.value = extractError(err)
  } finally {
    assigning.value = false
  }
}

async function handleUnassignDID(trunk: Trunk, didID: number) {
  if (!confirm('Remove DID from this trunk?')) return
  try {
    await trunksApi.unassignDID(trunk.id, { did_id: didID })
    await loadDIDs()
  } catch (err: unknown) {
    error.value = extractError(err)
  }
}

function didsForTrunk(trunkID: number): DID[] {
  return dids.value.filter((d: any) => d.trunk_id === trunkID)
}

function unassignedDIDs(): DID[] {
  return dids.value.filter((d: any) => !d.trunk_id)
}
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">SIP Trunks</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ trunks.length }} trunk{{ trunks.length !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadTrunks"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
        <button
          @click="syncFromTwilio"
          :disabled="syncing"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
        >
          <RefreshCw class="h-4 w-4 mr-2" />
          {{ syncing ? 'Syncing...' : 'Sync from Twilio' }}
        </button>
        <button
          @click="openCreateModal"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          <Plus class="h-4 w-4 mr-2" />
          New Trunk
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else class="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <div
        v-for="trunk in trunks"
        :key="trunk.id"
        class="bg-white dark:bg-gray-800 shadow rounded-lg p-6"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center">
            <div class="p-2 rounded-full bg-primary/10">
              <Server class="h-5 w-5 text-primary" />
            </div>
            <div class="ml-3">
              <p class="font-medium text-gray-900 dark:text-white">
                {{ trunk.friendly_name || trunk.twilio_sid }}
              </p>
              <p class="text-sm text-gray-500 font-mono">{{ trunk.twilio_sid }}</p>
            </div>
          </div>
          <div class="flex space-x-1">
            <button
              @click="openAssignModal(trunk)"
              class="p-1 text-gray-400 hover:text-primary"
              title="Assign DID"
            >
              <Phone class="h-4 w-4" />
            </button>
            <button
              @click="openEditModal(trunk)"
              class="p-1 text-gray-400 hover:text-primary"
            >
              <Edit2 class="h-4 w-4" />
            </button>
            <button
              @click="handleDelete(trunk)"
              class="p-1 text-gray-400 hover:text-destructive"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-2">
          <span
            v-if="trunk.secure"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
          >
            <Shield class="h-3 w-3 mr-1" />
            TLS Secure
          </span>
          <span
            v-else
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
          >
            Unencrypted
          </span>
          <span
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
          >
            {{ trunk.transfer_mode }}
          </span>
        </div>

        <div v-if="didsForTrunk(trunk.id).length > 0" class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <p class="text-xs font-medium text-gray-500 uppercase mb-2">Assigned DIDs</p>
          <div class="space-y-1">
            <div
              v-for="did in didsForTrunk(trunk.id)"
              :key="did.id"
              class="flex items-center justify-between text-sm"
            >
              <span class="text-gray-700 dark:text-gray-300">{{ did.phone_number }}</span>
              <button
                @click="handleUnassignDID(trunk, did.id)"
                class="text-xs text-destructive hover:underline"
              >
                Remove
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="trunks.length === 0" class="col-span-full text-center py-12 text-gray-500">
        No SIP trunks configured. Click "Sync from Twilio" to import existing trunks or "New Trunk" to create one.
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            {{ editingTrunk ? 'Edit Trunk' : 'New SIP Trunk' }}
          </h3>

          <form @submit.prevent="handleSubmit" class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Friendly Name
              </label>
              <input
                v-model="form.friendly_name"
                type="text"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="Main Trunk"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Transfer Mode
              </label>
              <select
                v-model="form.transfer_mode"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              >
                <option
                  v-for="mode in transferModes"
                  :key="mode.value"
                  :value="mode.value"
                >
                  {{ mode.label }}
                </option>
              </select>
            </div>

            <div class="flex items-center space-x-4">
              <label class="flex items-center">
                <input
                  v-model="form.secure"
                  type="checkbox"
                  class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                />
                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">TLS Secure</span>
              </label>

              <label class="flex items-center">
                <input
                  v-model="form.cnam_lookup_enabled"
                  type="checkbox"
                  class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                />
                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">CNAM Lookup</span>
              </label>
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
                {{ saving ? 'Saving...' : (editingTrunk ? 'Update' : 'Create') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Assign DID Modal -->
    <div v-if="showAssignModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showAssignModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Assign DID to {{ assigningTrunk?.friendly_name || assigningTrunk?.twilio_sid }}
          </h3>

          <div v-if="unassignedDIDs().length === 0" class="text-gray-500 text-center py-6">
            No unassigned DIDs available.
          </div>

          <div v-else class="space-y-2 max-h-64 overflow-y-auto">
            <button
              v-for="did in unassignedDIDs()"
              :key="did.id"
              @click="handleAssignDID(did.id)"
              :disabled="assigning"
              class="w-full flex items-center justify-between px-4 py-3 text-left border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
            >
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ did.phone_number }}</p>
                <p v-if="did.friendly_name" class="text-sm text-gray-500">{{ did.friendly_name }}</p>
              </div>
              <span class="text-sm text-primary font-medium">Assign</span>
            </button>
          </div>

          <div class="flex justify-end mt-4">
            <button
              @click="showAssignModal = false"
              class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

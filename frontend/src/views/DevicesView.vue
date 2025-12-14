<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { devicesApi, type Device } from '@/api/devices'
import { Monitor, Plus, Edit2, Trash2, Phone, PhoneOff, RefreshCw } from 'lucide-vue-next'

const devices = ref<Device[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const showModal = ref(false)
const editingDevice = ref<Device | null>(null)
const saving = ref(false)

const form = ref({
  name: '',
  extension: '',
  password: '',
  caller_id: ''
})

onMounted(async () => {
  await loadDevices()
})

async function loadDevices() {
  loading.value = true
  error.value = null
  try {
    const response = await devicesApi.list()
    // Handle paginated response - data can be null when empty
    devices.value = response?.data || []
  } catch {
    error.value = 'Failed to load devices'
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  editingDevice.value = null
  form.value = { name: '', extension: '', password: '', caller_id: '' }
  showModal.value = true
}

function openEditModal(device: Device) {
  editingDevice.value = device
  form.value = {
    name: device.name,
    extension: device.extension,
    password: '',
    caller_id: device.caller_id || ''
  }
  showModal.value = true
}

async function handleSubmit() {
  saving.value = true
  error.value = null

  try {
    if (editingDevice.value) {
      await devicesApi.update(editingDevice.value.id, form.value)
    } else {
      await devicesApi.create(form.value)
    }
    showModal.value = false
    await loadDevices()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Operation failed'
  } finally {
    saving.value = false
  }
}

async function handleDelete(device: Device) {
  if (!confirm(`Delete device "${device.name}"?`)) return

  try {
    await devicesApi.delete(device.id)
    await loadDevices()
  } catch {
    error.value = 'Failed to delete device'
  }
}

const registeredCount = computed(() => devices.value.filter(d => d.registered).length)
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Devices</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ registeredCount }} of {{ devices.length }} devices registered
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadDevices"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
        <button
          @click="openCreateModal"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          <Plus class="h-4 w-4 mr-2" />
          Add Device
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else class="mt-6 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead class="bg-gray-50 dark:bg-gray-700">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Device
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Extension
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Status
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              User Agent
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr v-for="device in devices" :key="device.id" class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <Monitor class="h-5 w-5 text-gray-400 mr-3" />
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">{{ device.name }}</div>
                  <div v-if="device.caller_id" class="text-sm text-gray-500">{{ device.caller_id }}</div>
                </div>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
              {{ device.extension }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  device.registered
                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
                ]"
              >
                <component :is="device.registered ? Phone : PhoneOff" class="h-3 w-3 mr-1" />
                {{ device.registered ? 'Online' : 'Offline' }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ device.user_agent || '-' }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <button
                @click="openEditModal(device)"
                class="text-primary hover:text-primary/80 mr-3"
              >
                <Edit2 class="h-4 w-4" />
              </button>
              <button
                @click="handleDelete(device)"
                class="text-destructive hover:text-destructive/80"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </td>
          </tr>
          <tr v-if="devices.length === 0">
            <td colspan="5" class="px-6 py-12 text-center text-gray-500">
              No devices configured. Click "Add Device" to create one.
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            {{ editingDevice ? 'Edit Device' : 'Add Device' }}
          </h3>

          <form @submit.prevent="handleSubmit" class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Device Name
              </label>
              <input
                v-model="form.name"
                type="text"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="Living Room Phone"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Extension
              </label>
              <input
                v-model="form.extension"
                type="text"
                required
                pattern="[0-9]+"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="101"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Password {{ editingDevice ? '(leave blank to keep current)' : '' }}
              </label>
              <input
                v-model="form.password"
                type="password"
                :required="!editingDevice"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="••••••••"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Caller ID (optional)
              </label>
              <input
                v-model="form.caller_id"
                type="text"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="+15551234567"
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
                {{ saving ? 'Saving...' : (editingDevice ? 'Update' : 'Create') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

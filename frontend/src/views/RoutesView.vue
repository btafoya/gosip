<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Route, Plus, Edit2, Trash2, RefreshCw, ArrowRight, Clock, User, Phone } from 'lucide-vue-next'
import api from '@/api/client'

interface RouteRule {
  id: number
  name: string
  priority: number
  enabled: boolean
  conditions: {
    did_id?: number
    caller_pattern?: string
    time_start?: string
    time_end?: string
    days_of_week?: number[]
  }
  action: 'ring' | 'forward' | 'voicemail' | 'reject'
  action_target?: string
  ring_timeout: number
  created_at: string
}

interface DID {
  id: number
  phone_number: string
  friendly_name: string
}

interface Device {
  id: number
  name: string
  extension: string
}

const routes = ref<RouteRule[]>([])
const dids = ref<DID[]>([])
const devices = ref<Device[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const showModal = ref(false)
const editingRoute = ref<RouteRule | null>(null)
const saving = ref(false)

const daysOfWeek = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const form = ref({
  name: '',
  priority: 10,
  enabled: true,
  did_id: null as number | null,
  caller_pattern: '',
  time_start: '',
  time_end: '',
  days_of_week: [] as number[],
  action: 'ring' as 'ring' | 'forward' | 'voicemail' | 'reject',
  action_target: '',
  ring_timeout: 30
})

onMounted(async () => {
  await Promise.all([loadRoutes(), loadDIDs(), loadDevices()])
})

async function loadRoutes() {
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/routes')
    routes.value = response.data.data || []
  } catch {
    error.value = 'Failed to load routes'
  } finally {
    loading.value = false
  }
}

async function loadDIDs() {
  try {
    const response = await api.get('/dids')
    dids.value = response.data.data || []
  } catch {
    console.error('Failed to load DIDs')
  }
}

async function loadDevices() {
  try {
    const response = await api.get('/devices')
    devices.value = response.data.data || []
  } catch {
    console.error('Failed to load devices')
  }
}

function openCreateModal() {
  editingRoute.value = null
  form.value = {
    name: '',
    priority: 10,
    enabled: true,
    did_id: null,
    caller_pattern: '',
    time_start: '',
    time_end: '',
    days_of_week: [],
    action: 'ring',
    action_target: '',
    ring_timeout: 30
  }
  showModal.value = true
}

function openEditModal(route: RouteRule) {
  editingRoute.value = route
  form.value = {
    name: route.name,
    priority: route.priority,
    enabled: route.enabled,
    did_id: route.conditions.did_id || null,
    caller_pattern: route.conditions.caller_pattern || '',
    time_start: route.conditions.time_start || '',
    time_end: route.conditions.time_end || '',
    days_of_week: route.conditions.days_of_week || [],
    action: route.action,
    action_target: route.action_target || '',
    ring_timeout: route.ring_timeout
  }
  showModal.value = true
}

async function handleSubmit() {
  saving.value = true
  error.value = null

  const payload = {
    name: form.value.name,
    priority: form.value.priority,
    enabled: form.value.enabled,
    conditions: {
      did_id: form.value.did_id || undefined,
      caller_pattern: form.value.caller_pattern || undefined,
      time_start: form.value.time_start || undefined,
      time_end: form.value.time_end || undefined,
      days_of_week: form.value.days_of_week.length > 0 ? form.value.days_of_week : undefined
    },
    action: form.value.action,
    action_target: form.value.action_target || undefined,
    ring_timeout: form.value.ring_timeout
  }

  try {
    if (editingRoute.value) {
      await api.put(`/routes/${editingRoute.value.id}`, payload)
    } else {
      await api.post('/routes', payload)
    }
    showModal.value = false
    await loadRoutes()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Operation failed'
  } finally {
    saving.value = false
  }
}

async function handleDelete(route: RouteRule) {
  if (!confirm(`Delete route "${route.name}"?`)) return

  try {
    await api.delete(`/routes/${route.id}`)
    await loadRoutes()
  } catch {
    error.value = 'Failed to delete route'
  }
}

async function toggleEnabled(route: RouteRule) {
  try {
    await api.put(`/routes/${route.id}`, { enabled: !route.enabled })
    await loadRoutes()
  } catch {
    error.value = 'Failed to update route'
  }
}

function getDIDName(didId: number | undefined): string {
  if (!didId) return 'Any'
  const did = dids.value.find(d => d.id === didId)
  return did?.friendly_name || did?.phone_number || 'Unknown'
}

function getActionDescription(route: RouteRule): string {
  switch (route.action) {
    case 'ring':
      return 'Ring all devices'
    case 'forward':
      return `Forward to ${route.action_target}`
    case 'voicemail':
      return 'Send to voicemail'
    case 'reject':
      return 'Reject call'
    default:
      return route.action
  }
}
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Call Routing</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ routes.length }} routing rule{{ routes.length !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadRoutes"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
        <button
          @click="openCreateModal"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          <Plus class="h-4 w-4 mr-2" />
          Add Rule
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else class="mt-6 space-y-4">
      <div
        v-for="route in routes"
        :key="route.id"
        :class="[
          'bg-white dark:bg-gray-800 shadow rounded-lg p-6',
          !route.enabled && 'opacity-60'
        ]"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center">
            <div class="p-2 rounded-full bg-primary/10">
              <Route class="h-5 w-5 text-primary" />
            </div>
            <div class="ml-3">
              <div class="flex items-center space-x-2">
                <p class="font-medium text-gray-900 dark:text-white">{{ route.name }}</p>
                <span class="text-xs text-gray-500">Priority: {{ route.priority }}</span>
              </div>
              <p class="text-sm text-gray-500 flex items-center mt-1">
                <ArrowRight class="h-3 w-3 mr-1" />
                {{ getActionDescription(route) }}
              </p>
            </div>
          </div>
          <div class="flex items-center space-x-2">
            <button
              @click="toggleEnabled(route)"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                route.enabled ? 'bg-primary' : 'bg-gray-200 dark:bg-gray-600'
              ]"
            >
              <span
                :class="[
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  route.enabled ? 'translate-x-6' : 'translate-x-1'
                ]"
              />
            </button>
            <button
              @click="openEditModal(route)"
              class="p-1 text-gray-400 hover:text-primary"
            >
              <Edit2 class="h-4 w-4" />
            </button>
            <button
              @click="handleDelete(route)"
              class="p-1 text-gray-400 hover:text-destructive"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-2">
          <span
            v-if="route.conditions.did_id"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
          >
            <Phone class="h-3 w-3 mr-1" />
            {{ getDIDName(route.conditions.did_id) }}
          </span>
          <span
            v-if="route.conditions.caller_pattern"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
          >
            <User class="h-3 w-3 mr-1" />
            {{ route.conditions.caller_pattern }}
          </span>
          <span
            v-if="route.conditions.time_start && route.conditions.time_end"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
          >
            <Clock class="h-3 w-3 mr-1" />
            {{ route.conditions.time_start }} - {{ route.conditions.time_end }}
          </span>
          <span
            v-if="route.conditions.days_of_week?.length"
            class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200"
          >
            {{ route.conditions.days_of_week.map(d => daysOfWeek[d]).join(', ') }}
          </span>
        </div>
      </div>

      <div v-if="routes.length === 0" class="text-center py-12 text-gray-500 bg-white dark:bg-gray-800 rounded-lg">
        No routing rules configured. Click "Add Rule" to create one.
      </div>
    </div>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full p-6 max-h-[90vh] overflow-y-auto">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            {{ editingRoute ? 'Edit Rule' : 'Add Rule' }}
          </h3>

          <form @submit.prevent="handleSubmit" class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div class="col-span-2">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Rule Name
                </label>
                <input
                  v-model="form.name"
                  type="text"
                  required
                  class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  placeholder="Business Hours"
                />
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Priority
                </label>
                <input
                  v-model.number="form.priority"
                  type="number"
                  min="1"
                  max="100"
                  class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                />
              </div>

              <div class="flex items-center pt-6">
                <input
                  v-model="form.enabled"
                  type="checkbox"
                  class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                />
                <label class="ml-2 text-sm text-gray-700 dark:text-gray-300">Enabled</label>
              </div>
            </div>

            <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">Conditions</h4>

              <div class="space-y-3">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Phone Number (DID)
                  </label>
                  <select
                    v-model="form.did_id"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  >
                    <option :value="null">Any</option>
                    <option v-for="did in dids" :key="did.id" :value="did.id">
                      {{ did.friendly_name || did.phone_number }}
                    </option>
                  </select>
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Caller Pattern (regex)
                  </label>
                  <input
                    v-model="form.caller_pattern"
                    type="text"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                    placeholder="^\+1555.*"
                  />
                </div>

                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      Time Start
                    </label>
                    <input
                      v-model="form.time_start"
                      type="time"
                      class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                    />
                  </div>
                  <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      Time End
                    </label>
                    <input
                      v-model="form.time_end"
                      type="time"
                      class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                    />
                  </div>
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Days of Week
                  </label>
                  <div class="flex flex-wrap gap-2">
                    <button
                      v-for="(day, index) in daysOfWeek"
                      :key="index"
                      type="button"
                      @click="form.days_of_week.includes(index)
                        ? form.days_of_week = form.days_of_week.filter(d => d !== index)
                        : form.days_of_week.push(index)"
                      :class="[
                        'px-3 py-1 text-sm rounded-md border',
                        form.days_of_week.includes(index)
                          ? 'bg-primary text-white border-primary'
                          : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600'
                      ]"
                    >
                      {{ day }}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">Action</h4>

              <div class="space-y-3">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Action Type
                  </label>
                  <select
                    v-model="form.action"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  >
                    <option value="ring">Ring Devices</option>
                    <option value="forward">Forward to Number</option>
                    <option value="voicemail">Send to Voicemail</option>
                    <option value="reject">Reject Call</option>
                  </select>
                </div>

                <div v-if="form.action === 'forward'">
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Forward To
                  </label>
                  <input
                    v-model="form.action_target"
                    type="tel"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                    placeholder="+15551234567"
                  />
                </div>

                <div v-if="form.action === 'ring'">
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Ring Timeout (seconds)
                  </label>
                  <input
                    v-model.number="form.ring_timeout"
                    type="number"
                    min="10"
                    max="120"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
              </div>
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
                {{ saving ? 'Saving...' : (editingRoute ? 'Update' : 'Create') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>
</script>

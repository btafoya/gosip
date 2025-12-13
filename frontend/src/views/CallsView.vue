<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Phone, PhoneIncoming, PhoneOutgoing, PhoneMissed, RefreshCw, Download, Filter } from 'lucide-vue-next'
import api from '@/api/client'

interface CDR {
  id: number
  call_sid: string
  direction: 'inbound' | 'outbound'
  from_number: string
  to_number: string
  status: 'completed' | 'busy' | 'no-answer' | 'canceled' | 'failed'
  duration: number
  start_time: string
  end_time?: string
  recording_url?: string
  recording_duration?: number
}

interface Pagination {
  page: number
  per_page: number
  total: number
  total_pages: number
}

const cdrs = ref<CDR[]>([])
const pagination = ref<Pagination>({ page: 1, per_page: 20, total: 0, total_pages: 0 })
const loading = ref(true)
const error = ref<string | null>(null)
const showFilters = ref(false)

const filters = ref({
  direction: '' as '' | 'inbound' | 'outbound',
  status: '' as '' | 'completed' | 'busy' | 'no-answer' | 'canceled' | 'failed',
  from_date: '',
  to_date: ''
})

onMounted(async () => {
  await loadCDRs()
})

async function loadCDRs(page = 1) {
  loading.value = true
  error.value = null

  try {
    const params = new URLSearchParams()
    params.set('page', String(page))
    params.set('per_page', '20')

    if (filters.value.direction) params.set('direction', filters.value.direction)
    if (filters.value.status) params.set('status', filters.value.status)
    if (filters.value.from_date) params.set('from_date', filters.value.from_date)
    if (filters.value.to_date) params.set('to_date', filters.value.to_date)

    const response = await api.get(`/cdrs?${params.toString()}`)
    cdrs.value = response.data.data || []
    pagination.value = response.data.pagination || { page: 1, per_page: 20, total: 0, total_pages: 0 }
  } catch {
    error.value = 'Failed to load call history'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  loadCDRs(1)
  showFilters.value = false
}

function clearFilters() {
  filters.value = { direction: '', status: '', from_date: '', to_date: '' }
  loadCDRs(1)
  showFilters.value = false
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins < 60) return `${mins}m ${secs}s`
  const hours = Math.floor(mins / 60)
  const remainingMins = mins % 60
  return `${hours}h ${remainingMins}m`
}

function formatDateTime(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleString()
}

function formatPhoneNumber(phone: string): string {
  if (phone.startsWith('+1') && phone.length === 12) {
    return `(${phone.slice(2, 5)}) ${phone.slice(5, 8)}-${phone.slice(8)}`
  }
  return phone
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'completed': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
    case 'busy': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
    case 'no-answer': return 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200'
    case 'canceled': return 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
    case 'failed': return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
  }
}

function getDirectionIcon(direction: string, status: string) {
  if (status === 'no-answer' || status === 'canceled') return PhoneMissed
  return direction === 'inbound' ? PhoneIncoming : PhoneOutgoing
}

const hasActiveFilters = computed(() => {
  return filters.value.direction || filters.value.status || filters.value.from_date || filters.value.to_date
})
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Call History</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ pagination.total }} total call{{ pagination.total !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="showFilters = !showFilters"
          :class="[
            'flex items-center px-3 py-2 text-sm rounded-md',
            hasActiveFilters
              ? 'bg-primary text-white'
              : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
          ]"
        >
          <Filter class="h-4 w-4 mr-1" />
          Filters
        </button>
        <button
          @click="loadCDRs(pagination.page)"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- Filters Panel -->
    <div v-if="showFilters" class="mt-4 bg-white dark:bg-gray-800 shadow rounded-lg p-4">
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Direction</label>
          <select
            v-model="filters.direction"
            class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
          >
            <option value="">All</option>
            <option value="inbound">Inbound</option>
            <option value="outbound">Outbound</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Status</label>
          <select
            v-model="filters.status"
            class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
          >
            <option value="">All</option>
            <option value="completed">Completed</option>
            <option value="busy">Busy</option>
            <option value="no-answer">No Answer</option>
            <option value="canceled">Canceled</option>
            <option value="failed">Failed</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">From Date</label>
          <input
            v-model="filters.from_date"
            type="date"
            class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">To Date</label>
          <input
            v-model="filters.to_date"
            type="date"
            class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
          />
        </div>
      </div>
      <div class="mt-4 flex justify-end space-x-2">
        <button
          @click="clearFilters"
          class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
        >
          Clear
        </button>
        <button
          @click="applyFilters"
          class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          Apply
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
              Call
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              From / To
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Status
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Duration
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Date
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Recording
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr v-for="cdr in cdrs" :key="cdr.id" class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <component
                  :is="getDirectionIcon(cdr.direction, cdr.status)"
                  :class="[
                    'h-5 w-5 mr-3',
                    cdr.direction === 'inbound' ? 'text-blue-500' : 'text-green-500',
                    (cdr.status === 'no-answer' || cdr.status === 'canceled') && 'text-orange-500'
                  ]"
                />
                <span class="text-sm text-gray-900 dark:text-white capitalize">
                  {{ cdr.direction }}
                </span>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="text-sm">
                <p class="text-gray-900 dark:text-white">{{ formatPhoneNumber(cdr.from_number) }}</p>
                <p class="text-gray-500">to {{ formatPhoneNumber(cdr.to_number) }}</p>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                :class="[
                  'inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium capitalize',
                  getStatusColor(cdr.status)
                ]"
              >
                {{ cdr.status.replace('-', ' ') }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
              {{ cdr.duration > 0 ? formatDuration(cdr.duration) : '-' }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDateTime(cdr.start_time) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right">
              <a
                v-if="cdr.recording_url"
                :href="cdr.recording_url"
                target="_blank"
                class="text-primary hover:text-primary/80"
              >
                <Download class="h-4 w-4" />
              </a>
              <span v-else class="text-gray-400">-</span>
            </td>
          </tr>
          <tr v-if="cdrs.length === 0">
            <td colspan="6" class="px-6 py-12 text-center text-gray-500">
              No calls found.
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div v-if="pagination.total_pages > 1" class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-between items-center">
        <p class="text-sm text-gray-500">
          Page {{ pagination.page }} of {{ pagination.total_pages }}
        </p>
        <div class="flex space-x-2">
          <button
            @click="loadCDRs(pagination.page - 1)"
            :disabled="pagination.page === 1"
            class="px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
          >
            Previous
          </button>
          <button
            @click="loadCDRs(pagination.page + 1)"
            :disabled="pagination.page === pagination.total_pages"
            class="px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
</script>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import {
  Phone,
  PhoneIncoming,
  PhoneOutgoing,
  PhoneOff,
  Pause,
  Play,
  ArrowRightLeft,
  RefreshCw,
  Music,
  X,
  AlertCircle,
  Clock
} from 'lucide-vue-next'
import callsApi, { type ActiveCall, type MOHStatus } from '@/api/calls'

const calls = ref<ActiveCall[]>([])
const mohStatus = ref<MOHStatus | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const refreshInterval = ref<ReturnType<typeof setInterval> | null>(null)

// Transfer dialog state
const showTransferDialog = ref(false)
const transferCallId = ref<string | null>(null)
const transferType = ref<'blind' | 'attended'>('blind')
const transferTarget = ref('')
const transferConsultId = ref('')

// MOH settings dialog
const showMOHDialog = ref(false)
const mohEnabled = ref(false)
const mohAudioPath = ref('')

onMounted(async () => {
  await Promise.all([loadCalls(), loadMOHStatus()])
  // Auto-refresh every 2 seconds
  refreshInterval.value = setInterval(() => {
    loadCalls()
  }, 2000)
})

onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})

async function loadCalls() {
  try {
    const response = await callsApi.listActiveCalls()
    calls.value = response.data
    error.value = null
  } catch {
    if (loading.value) {
      error.value = 'Failed to load active calls'
    }
  } finally {
    loading.value = false
  }
}

async function loadMOHStatus() {
  try {
    const response = await callsApi.getMOHStatus()
    mohStatus.value = response.data
    mohEnabled.value = response.data.enabled
    mohAudioPath.value = response.data.audio_path
  } catch {
    // MOH status is optional
  }
}

async function holdCall(callId: string, hold: boolean) {
  actionLoading.value = callId
  try {
    await callsApi.holdCall(callId, hold)
    await loadCalls()
  } catch (err) {
    error.value = `Failed to ${hold ? 'hold' : 'resume'} call`
  } finally {
    actionLoading.value = null
  }
}

async function hangupCall(callId: string) {
  if (!confirm('Are you sure you want to end this call?')) return

  actionLoading.value = callId
  try {
    await callsApi.hangupCall(callId)
    await loadCalls()
  } catch {
    error.value = 'Failed to end call'
  } finally {
    actionLoading.value = null
  }
}

function openTransferDialog(callId: string) {
  transferCallId.value = callId
  transferType.value = 'blind'
  transferTarget.value = ''
  transferConsultId.value = ''
  showTransferDialog.value = true
}

async function executeTransfer() {
  if (!transferCallId.value) return

  actionLoading.value = transferCallId.value
  try {
    if (transferType.value === 'blind') {
      await callsApi.transferCall(transferCallId.value, 'blind', transferTarget.value)
    } else {
      await callsApi.transferCall(transferCallId.value, 'attended', undefined, transferConsultId.value)
    }
    showTransferDialog.value = false
    await loadCalls()
  } catch {
    error.value = 'Failed to transfer call'
  } finally {
    actionLoading.value = null
  }
}

async function cancelTransfer(callId: string) {
  actionLoading.value = callId
  try {
    await callsApi.cancelTransfer(callId)
    await loadCalls()
  } catch {
    error.value = 'Failed to cancel transfer'
  } finally {
    actionLoading.value = null
  }
}

function openMOHDialog() {
  if (mohStatus.value) {
    mohEnabled.value = mohStatus.value.enabled
    mohAudioPath.value = mohStatus.value.audio_path
  }
  showMOHDialog.value = true
}

async function updateMOHSettings() {
  try {
    const response = await callsApi.updateMOH({
      enabled: mohEnabled.value,
      audio_path: mohAudioPath.value
    })
    mohStatus.value = response.data
    showMOHDialog.value = false
  } catch {
    error.value = 'Failed to update MOH settings'
  }
}

function formatDuration(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
}

function formatPhoneNumber(phone: string): string {
  if (phone.startsWith('+1') && phone.length === 12) {
    return `(${phone.slice(2, 5)}) ${phone.slice(5, 8)}-${phone.slice(8)}`
  }
  return phone
}

function getStateColor(state: string): string {
  switch (state) {
    case 'active': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
    case 'ringing': return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
    case 'held': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
    case 'holding': return 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200'
    case 'transferring': return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
    case 'terminated': return 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
  }
}

function getDirectionIcon(direction: string) {
  return direction === 'inbound' ? PhoneIncoming : PhoneOutgoing
}

// Get other active calls for attended transfer selection
const otherActiveCalls = computed(() => {
  if (!transferCallId.value) return []
  return calls.value.filter(c =>
    c.call_id !== transferCallId.value &&
    (c.state === 'active' || c.state === 'holding')
  )
})

const activeCallCount = computed(() => calls.value.length)
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Call Control</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ activeCallCount }} active call{{ activeCallCount !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="openMOHDialog"
          class="flex items-center px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
          :class="{ 'bg-primary/10 text-primary': mohStatus?.enabled }"
        >
          <Music class="h-4 w-4 mr-1" />
          MOH {{ mohStatus?.enabled ? 'On' : 'Off' }}
        </button>
        <button
          @click="loadCalls"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md flex items-center">
      <AlertCircle class="h-4 w-4 mr-2" />
      {{ error }}
      <button @click="error = null" class="ml-auto">
        <X class="h-4 w-4" />
      </button>
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <!-- Active Calls Grid -->
    <div v-else-if="calls.length > 0" class="mt-6 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="call in calls"
        :key="call.call_id"
        class="bg-white dark:bg-gray-800 shadow rounded-lg p-4 border-l-4"
        :class="{
          'border-green-500': call.state === 'active',
          'border-blue-500': call.state === 'ringing',
          'border-yellow-500': call.state === 'held' || call.state === 'holding',
          'border-purple-500': call.state === 'transferring',
          'border-gray-500': call.state === 'terminated'
        }"
      >
        <!-- Call Header -->
        <div class="flex items-center justify-between mb-3">
          <div class="flex items-center">
            <component
              :is="getDirectionIcon(call.direction)"
              :class="[
                'h-5 w-5 mr-2',
                call.direction === 'inbound' ? 'text-blue-500' : 'text-green-500'
              ]"
            />
            <span class="font-medium text-gray-900 dark:text-white capitalize">
              {{ call.direction }}
            </span>
          </div>
          <span
            :class="[
              'inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium capitalize',
              getStateColor(call.state)
            ]"
          >
            {{ call.state }}
          </span>
        </div>

        <!-- Call Details -->
        <div class="space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-gray-500">From:</span>
            <span class="text-gray-900 dark:text-white font-medium">
              {{ formatPhoneNumber(call.from_number) }}
            </span>
          </div>
          <div class="flex justify-between">
            <span class="text-gray-500">To:</span>
            <span class="text-gray-900 dark:text-white font-medium">
              {{ formatPhoneNumber(call.to_number) }}
            </span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-500">Duration:</span>
            <span class="text-gray-900 dark:text-white font-mono flex items-center">
              <Clock class="h-3 w-3 mr-1" />
              {{ formatDuration(call.duration) }}
            </span>
          </div>
          <div v-if="call.transfer_target" class="flex justify-between">
            <span class="text-gray-500">Transfer to:</span>
            <span class="text-purple-600 dark:text-purple-400 font-medium">
              {{ call.transfer_target }}
            </span>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="mt-4 pt-3 border-t border-gray-200 dark:border-gray-700 flex flex-wrap gap-2">
          <!-- Hold/Resume -->
          <button
            v-if="call.state === 'active'"
            @click="holdCall(call.call_id, true)"
            :disabled="actionLoading === call.call_id"
            class="flex items-center px-3 py-1.5 text-sm bg-yellow-100 text-yellow-700 hover:bg-yellow-200 dark:bg-yellow-900 dark:text-yellow-300 rounded-md disabled:opacity-50"
          >
            <Pause class="h-4 w-4 mr-1" />
            Hold
          </button>
          <button
            v-if="call.state === 'holding' || call.state === 'held'"
            @click="holdCall(call.call_id, false)"
            :disabled="actionLoading === call.call_id"
            class="flex items-center px-3 py-1.5 text-sm bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900 dark:text-green-300 rounded-md disabled:opacity-50"
          >
            <Play class="h-4 w-4 mr-1" />
            Resume
          </button>

          <!-- Transfer -->
          <button
            v-if="call.state === 'active' || call.state === 'holding'"
            @click="openTransferDialog(call.call_id)"
            :disabled="actionLoading === call.call_id"
            class="flex items-center px-3 py-1.5 text-sm bg-purple-100 text-purple-700 hover:bg-purple-200 dark:bg-purple-900 dark:text-purple-300 rounded-md disabled:opacity-50"
          >
            <ArrowRightLeft class="h-4 w-4 mr-1" />
            Transfer
          </button>

          <!-- Cancel Transfer -->
          <button
            v-if="call.state === 'transferring'"
            @click="cancelTransfer(call.call_id)"
            :disabled="actionLoading === call.call_id"
            class="flex items-center px-3 py-1.5 text-sm bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 rounded-md disabled:opacity-50"
          >
            <X class="h-4 w-4 mr-1" />
            Cancel
          </button>

          <!-- Hangup -->
          <button
            v-if="call.state !== 'terminated'"
            @click="hangupCall(call.call_id)"
            :disabled="actionLoading === call.call_id"
            class="flex items-center px-3 py-1.5 text-sm bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900 dark:text-red-300 rounded-md disabled:opacity-50 ml-auto"
          >
            <PhoneOff class="h-4 w-4 mr-1" />
            End
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="mt-6 bg-white dark:bg-gray-800 shadow rounded-lg p-12 text-center">
      <Phone class="h-12 w-12 mx-auto text-gray-400 mb-4" />
      <h3 class="text-lg font-medium text-gray-900 dark:text-white">No Active Calls</h3>
      <p class="mt-1 text-sm text-gray-500">
        Active calls will appear here when they come in or are placed.
      </p>
    </div>

    <!-- MOH Status Card -->
    <div v-if="mohStatus" class="mt-6 bg-white dark:bg-gray-800 shadow rounded-lg p-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center">
          <Music class="h-5 w-5 text-primary mr-2" />
          <span class="font-medium text-gray-900 dark:text-white">Music on Hold</span>
        </div>
        <span
          :class="[
            'inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium',
            mohStatus.enabled
              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
              : 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
          ]"
        >
          {{ mohStatus.enabled ? 'Enabled' : 'Disabled' }}
        </span>
      </div>
      <div class="mt-2 text-sm text-gray-500">
        <p>Audio: {{ mohStatus.audio_path || 'Default' }}</p>
        <p>Active streams: {{ mohStatus.active_count }}</p>
      </div>
    </div>

    <!-- Transfer Dialog -->
    <div
      v-if="showTransferDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showTransferDialog = false"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 w-full max-w-md">
        <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
          Transfer Call
        </h3>

        <div class="space-y-4">
          <!-- Transfer Type -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Transfer Type
            </label>
            <div class="flex space-x-4">
              <label class="flex items-center">
                <input
                  type="radio"
                  v-model="transferType"
                  value="blind"
                  class="mr-2"
                />
                <span class="text-sm text-gray-700 dark:text-gray-300">Blind Transfer</span>
              </label>
              <label class="flex items-center">
                <input
                  type="radio"
                  v-model="transferType"
                  value="attended"
                  class="mr-2"
                  :disabled="otherActiveCalls.length === 0"
                />
                <span class="text-sm text-gray-700 dark:text-gray-300">Attended Transfer</span>
              </label>
            </div>
          </div>

          <!-- Blind Transfer Target -->
          <div v-if="transferType === 'blind'">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Transfer To
            </label>
            <input
              v-model="transferTarget"
              type="tel"
              placeholder="Phone number or extension"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
            />
          </div>

          <!-- Attended Transfer - Select Consult Call -->
          <div v-else>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Transfer To (Active Call)
            </label>
            <select
              v-model="transferConsultId"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
            >
              <option value="">Select a call...</option>
              <option
                v-for="c in otherActiveCalls"
                :key="c.call_id"
                :value="c.call_id"
              >
                {{ formatPhoneNumber(c.from_number) }} → {{ formatPhoneNumber(c.to_number) }}
              </option>
            </select>
            <p v-if="otherActiveCalls.length === 0" class="mt-1 text-sm text-gray-500">
              No other active calls available for attended transfer.
            </p>
          </div>
        </div>

        <div class="mt-6 flex justify-end space-x-3">
          <button
            @click="showTransferDialog = false"
            class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            Cancel
          </button>
          <button
            @click="executeTransfer"
            :disabled="(transferType === 'blind' && !transferTarget) || (transferType === 'attended' && !transferConsultId)"
            class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
          >
            Transfer
          </button>
        </div>
      </div>
    </div>

    <!-- MOH Settings Dialog -->
    <div
      v-if="showMOHDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showMOHDialog = false"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 w-full max-w-md">
        <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
          Music on Hold Settings
        </h3>

        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              Enable MOH
            </label>
            <button
              @click="mohEnabled = !mohEnabled"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                mohEnabled ? 'bg-primary' : 'bg-gray-200 dark:bg-gray-700'
              ]"
            >
              <span
                :class="[
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  mohEnabled ? 'translate-x-6' : 'translate-x-1'
                ]"
              />
            </button>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Audio File Path
            </label>
            <input
              v-model="mohAudioPath"
              type="text"
              placeholder="/var/lib/gosip/moh/default.wav"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
            />
            <p class="mt-1 text-xs text-gray-500">
              Leave empty for default audio
            </p>
          </div>
        </div>

        <div class="mt-6 flex justify-end space-x-3">
          <button
            @click="showMOHDialog = false"
            class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            Cancel
          </button>
          <button
            @click="updateMOHSettings"
            class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
          >
            Save
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

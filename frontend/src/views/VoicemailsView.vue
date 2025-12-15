<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Voicemail, Play, Pause, Trash2, RefreshCw, Mail, MailOpen, Download, Bell, BellOff, ChevronDown, ChevronUp, Phone, Send } from 'lucide-vue-next'
import api from '@/api/client'

interface VoicemailRecord {
  id: number
  call_sid: string
  from_number: string
  to_number: string
  duration: number
  recording_url: string
  transcription?: string
  transcription_status: 'pending' | 'completed' | 'failed'
  read: boolean
  created_at: string
}

interface MWIState {
  aor: string
  new_messages: number
  old_messages: number
  new_urgent: number
  old_urgent: number
  last_updated: string
}

interface MWISubscription {
  id: string
  aor: string
  contact_uri: string
  expires: number
  expires_at: string
}

interface MWIStatus {
  enabled: boolean
  subscription_count: number
  states: MWIState[]
  subscriptions: MWISubscription[]
}

const voicemails = ref<VoicemailRecord[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const playingId = ref<number | null>(null)
const audioRef = ref<HTMLAudioElement | null>(null)

// MWI state
const mwiStatus = ref<MWIStatus | null>(null)
const mwiLoading = ref(false)
const mwiExpanded = ref(false)
const mwiNotifying = ref(false)

onMounted(async () => {
  await Promise.all([loadVoicemails(), loadMWIStatus()])
})

async function loadVoicemails() {
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/voicemails')
    voicemails.value = response.data.data || []
  } catch {
    error.value = 'Failed to load voicemails'
  } finally {
    loading.value = false
  }
}

async function loadMWIStatus() {
  mwiLoading.value = true
  try {
    const response = await api.get('/mwi/status')
    mwiStatus.value = response.data.data || response.data
  } catch {
    // MWI status is optional, don't show error
    mwiStatus.value = null
  } finally {
    mwiLoading.value = false
  }
}

async function triggerMWINotification() {
  mwiNotifying.value = true
  try {
    await api.post('/mwi/notify')
    // Refresh status after notification
    await loadMWIStatus()
  } catch {
    error.value = 'Failed to trigger MWI notification'
  } finally {
    mwiNotifying.value = false
  }
}

async function toggleRead(vm: VoicemailRecord) {
  try {
    await api.put(`/voicemails/${vm.id}/read`)
    vm.read = !vm.read
    // Refresh MWI status since it may have changed
    await loadMWIStatus()
  } catch {
    error.value = 'Failed to update voicemail'
  }
}

async function deleteVoicemail(vm: VoicemailRecord) {
  if (!confirm('Delete this voicemail?')) return

  try {
    await api.delete(`/voicemails/${vm.id}`)
    voicemails.value = voicemails.value.filter(v => v.id !== vm.id)
    // Refresh MWI status since it may have changed
    await loadMWIStatus()
  } catch {
    error.value = 'Failed to delete voicemail'
  }
}

function playVoicemail(vm: VoicemailRecord) {
  if (playingId.value === vm.id) {
    // Stop playing
    if (audioRef.value) {
      audioRef.value.pause()
      audioRef.value = null
    }
    playingId.value = null
    return
  }

  // Stop any current playback
  if (audioRef.value) {
    audioRef.value.pause()
  }

  // Start new playback
  playingId.value = vm.id
  audioRef.value = new Audio(vm.recording_url)
  audioRef.value.play()
  audioRef.value.onended = () => {
    playingId.value = null
    audioRef.value = null

    // Mark as read after playing
    if (!vm.read) {
      toggleRead(vm)
    }
  }
}

function formatDuration(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
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

function extractUsername(aor: string): string {
  // Extract username from "sip:username@domain"
  const match = aor.match(/sip:([^@]+)@/)
  return match ? match[1] : aor
}

const unreadCount = computed(() => voicemails.value.filter(v => !v.read).length)
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Voicemails</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ unreadCount }} unread of {{ voicemails.length }} total
        </p>
      </div>
      <button
        @click="loadVoicemails"
        class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
      >
        <RefreshCw class="h-4 w-4" />
      </button>
    </div>

    <!-- MWI Status Panel -->
    <div class="mt-4 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
      <button
        @click="mwiExpanded = !mwiExpanded"
        class="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
      >
        <div class="flex items-center space-x-3">
          <div :class="[
            'p-2 rounded-full',
            mwiStatus?.enabled ? 'bg-green-100 dark:bg-green-900' : 'bg-gray-100 dark:bg-gray-700'
          ]">
            <component
              :is="mwiStatus?.enabled ? Bell : BellOff"
              :class="[
                'h-4 w-4',
                mwiStatus?.enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-400'
              ]"
            />
          </div>
          <div>
            <span class="font-medium text-gray-900 dark:text-white">Message Waiting Indicator (MWI)</span>
            <span v-if="mwiStatus" class="ml-2 text-sm text-gray-500">
              {{ mwiStatus.enabled ? `${mwiStatus.subscription_count} device(s) subscribed` : 'Disabled' }}
            </span>
          </div>
        </div>
        <component :is="mwiExpanded ? ChevronUp : ChevronDown" class="h-5 w-5 text-gray-400" />
      </button>

      <div v-if="mwiExpanded" class="border-t border-gray-200 dark:border-gray-700 px-4 py-4 space-y-4">
        <div v-if="mwiLoading" class="text-gray-500 text-sm">Loading MWI status...</div>

        <div v-else-if="!mwiStatus || !mwiStatus.enabled" class="text-center py-4">
          <BellOff class="h-8 w-8 mx-auto mb-2 text-gray-300" />
          <p class="text-sm text-gray-500">MWI is not available. The SIP server may not be running.</p>
        </div>

        <template v-else>
          <!-- Status Summary -->
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
              <div class="text-2xl font-bold text-primary">{{ mwiStatus.subscription_count }}</div>
              <div class="text-xs text-gray-500">Subscribed Devices</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
              <div class="text-2xl font-bold text-orange-500">{{ mwiStatus.states.length }}</div>
              <div class="text-xs text-gray-500">Active Mailboxes</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
              <div class="text-2xl font-bold text-red-500">{{ unreadCount }}</div>
              <div class="text-xs text-gray-500">Unread Messages</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
              <div class="text-2xl font-bold text-gray-500">{{ voicemails.length - unreadCount }}</div>
              <div class="text-xs text-gray-500">Read Messages</div>
            </div>
          </div>

          <!-- Subscriptions -->
          <div v-if="mwiStatus.subscriptions.length > 0">
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Active Subscriptions</h3>
            <div class="space-y-2">
              <div
                v-for="sub in mwiStatus.subscriptions"
                :key="sub.id"
                class="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 rounded-lg px-3 py-2"
              >
                <div class="flex items-center space-x-3">
                  <Phone class="h-4 w-4 text-gray-400" />
                  <div>
                    <span class="font-mono text-sm text-gray-900 dark:text-white">{{ extractUsername(sub.aor) }}</span>
                    <span class="ml-2 text-xs text-gray-500">→ {{ sub.contact_uri }}</span>
                  </div>
                </div>
                <div class="text-xs text-gray-500">
                  Expires: {{ sub.expires }}s
                </div>
              </div>
            </div>
          </div>

          <!-- Mailbox States -->
          <div v-if="mwiStatus.states.length > 0">
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Mailbox States</h3>
            <div class="space-y-2">
              <div
                v-for="state in mwiStatus.states"
                :key="state.aor"
                class="flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 rounded-lg px-3 py-2"
              >
                <div class="flex items-center space-x-3">
                  <Voicemail class="h-4 w-4 text-orange-500" />
                  <span class="font-mono text-sm text-gray-900 dark:text-white">{{ extractUsername(state.aor) }}</span>
                </div>
                <div class="flex items-center space-x-4 text-sm">
                  <span :class="state.new_messages > 0 ? 'text-red-500 font-medium' : 'text-gray-500'">
                    {{ state.new_messages }} new
                  </span>
                  <span class="text-gray-400">{{ state.old_messages }} old</span>
                </div>
              </div>
            </div>
          </div>

          <!-- No subscriptions message -->
          <div v-if="mwiStatus.subscriptions.length === 0" class="text-center py-4">
            <Phone class="h-8 w-8 mx-auto mb-2 text-gray-300" />
            <p class="text-sm text-gray-500">No devices are currently subscribed to MWI.</p>
            <p class="text-xs text-gray-400 mt-1">Phones will subscribe automatically when registered.</p>
          </div>

          <!-- Manual Trigger Button -->
          <div class="pt-2 border-t border-gray-200 dark:border-gray-700">
            <button
              @click="triggerMWINotification"
              :disabled="mwiNotifying || mwiStatus.subscriptions.length === 0"
              class="inline-flex items-center px-3 py-2 text-sm font-medium rounded-md text-white bg-primary hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Send class="h-4 w-4 mr-2" />
              {{ mwiNotifying ? 'Sending...' : 'Test MWI Notification' }}
            </button>
            <p class="mt-2 text-xs text-gray-500">
              Manually send MWI notifications to all subscribed devices for troubleshooting.
            </p>
          </div>
        </template>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else class="mt-6 space-y-4">
      <div
        v-for="vm in voicemails"
        :key="vm.id"
        :class="[
          'bg-white dark:bg-gray-800 shadow rounded-lg p-6',
          !vm.read && 'ring-2 ring-primary ring-opacity-50'
        ]"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-start">
            <div class="p-2 rounded-full bg-orange-100 dark:bg-orange-900">
              <Voicemail class="h-5 w-5 text-orange-600 dark:text-orange-400" />
            </div>
            <div class="ml-4">
              <div class="flex items-center space-x-2">
                <p class="font-medium text-gray-900 dark:text-white">
                  {{ formatPhoneNumber(vm.from_number) }}
                </p>
                <span
                  v-if="!vm.read"
                  class="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-primary text-white"
                >
                  New
                </span>
              </div>
              <p class="text-sm text-gray-500">
                {{ formatDateTime(vm.created_at) }} · {{ formatDuration(vm.duration) }}
              </p>
            </div>
          </div>

          <div class="flex items-center space-x-2">
            <button
              @click="playVoicemail(vm)"
              class="p-2 text-gray-400 hover:text-primary rounded-full hover:bg-gray-100 dark:hover:bg-gray-700"
              :title="playingId === vm.id ? 'Stop' : 'Play'"
            >
              <component :is="playingId === vm.id ? Pause : Play" class="h-5 w-5" />
            </button>
            <a
              :href="vm.recording_url"
              download
              class="p-2 text-gray-400 hover:text-primary rounded-full hover:bg-gray-100 dark:hover:bg-gray-700"
              title="Download"
            >
              <Download class="h-5 w-5" />
            </a>
            <button
              @click="toggleRead(vm)"
              class="p-2 text-gray-400 hover:text-primary rounded-full hover:bg-gray-100 dark:hover:bg-gray-700"
              :title="vm.read ? 'Mark as unread' : 'Mark as read'"
            >
              <component :is="vm.read ? Mail : MailOpen" class="h-5 w-5" />
            </button>
            <button
              @click="deleteVoicemail(vm)"
              class="p-2 text-gray-400 hover:text-destructive rounded-full hover:bg-gray-100 dark:hover:bg-gray-700"
              title="Delete"
            >
              <Trash2 class="h-5 w-5" />
            </button>
          </div>
        </div>

        <div v-if="vm.transcription" class="mt-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
          <p class="text-sm text-gray-700 dark:text-gray-300">
            <span class="font-medium">Transcription:</span>
            {{ vm.transcription }}
          </p>
        </div>

        <div v-else-if="vm.transcription_status === 'pending'" class="mt-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
          <p class="text-sm text-gray-500 italic">Transcription in progress...</p>
        </div>

        <div v-else-if="vm.transcription_status === 'failed'" class="mt-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
          <p class="text-sm text-gray-500 italic">Transcription failed</p>
        </div>
      </div>

      <div v-if="voicemails.length === 0" class="text-center py-12 text-gray-500 bg-white dark:bg-gray-800 rounded-lg">
        <Voicemail class="h-12 w-12 mx-auto mb-4 text-gray-300" />
        <p>No voicemails</p>
      </div>
    </div>
  </div>
</template>

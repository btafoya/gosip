<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Voicemail, Play, Pause, Trash2, RefreshCw, Mail, MailOpen, Download } from 'lucide-vue-next'
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

const voicemails = ref<VoicemailRecord[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const playingId = ref<number | null>(null)
const audioRef = ref<HTMLAudioElement | null>(null)

onMounted(async () => {
  await loadVoicemails()
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

async function toggleRead(vm: VoicemailRecord) {
  try {
    await api.put(`/voicemails/${vm.id}`, { read: !vm.read })
    vm.read = !vm.read
  } catch {
    error.value = 'Failed to update voicemail'
  }
}

async function deleteVoicemail(vm: VoicemailRecord) {
  if (!confirm('Delete this voicemail?')) return

  try {
    await api.delete(`/voicemails/${vm.id}`)
    voicemails.value = voicemails.value.filter(v => v.id !== vm.id)
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
</script>

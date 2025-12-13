<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { MessageSquare, Send, RefreshCw, ArrowLeft } from 'lucide-vue-next'
import api from '@/api/client'

interface Message {
  id: number
  direction: 'inbound' | 'outbound'
  from_number: string
  to_number: string
  body: string
  media_urls?: string[]
  status: string
  created_at: string
}

interface Conversation {
  phone_number: string
  last_message: string
  last_message_time: string
  unread_count: number
}

const conversations = ref<Conversation[]>([])
const messages = ref<Message[]>([])
const selectedConversation = ref<string | null>(null)
const loading = ref(true)
const sendingMessage = ref(false)
const error = ref<string | null>(null)
const newMessage = ref('')

const dids = ref<{ id: number; phone_number: string; friendly_name: string }[]>([])
const selectedDID = ref<string>('')

onMounted(async () => {
  await Promise.all([loadConversations(), loadDIDs()])
})

async function loadDIDs() {
  try {
    const response = await api.get('/dids')
    dids.value = response.data.data || []
    if (dids.value.length > 0) {
      selectedDID.value = dids.value[0].phone_number
    }
  } catch {
    console.error('Failed to load DIDs')
  }
}

async function loadConversations() {
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/messages/conversations')
    conversations.value = response.data.data || []
  } catch {
    error.value = 'Failed to load conversations'
  } finally {
    loading.value = false
  }
}

async function loadMessages(phoneNumber: string) {
  loading.value = true
  selectedConversation.value = phoneNumber
  try {
    const response = await api.get(`/messages/conversation/${encodeURIComponent(phoneNumber)}`)
    messages.value = response.data.data || []
  } catch {
    error.value = 'Failed to load messages'
  } finally {
    loading.value = false
  }
}

async function sendMessage() {
  if (!newMessage.value.trim() || !selectedConversation.value || !selectedDID.value) return

  sendingMessage.value = true
  try {
    await api.post('/messages', {
      to: selectedConversation.value,
      from: selectedDID.value,
      body: newMessage.value
    })
    newMessage.value = ''
    await loadMessages(selectedConversation.value)
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to send message'
  } finally {
    sendingMessage.value = false
  }
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (diffDays === 1) {
    return 'Yesterday'
  } else if (diffDays < 7) {
    return date.toLocaleDateString([], { weekday: 'short' })
  }
  return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function formatPhoneNumber(phone: string): string {
  if (phone.startsWith('+1') && phone.length === 12) {
    return `(${phone.slice(2, 5)}) ${phone.slice(5, 8)}-${phone.slice(8)}`
  }
  return phone
}

const sortedMessages = computed(() => {
  return [...messages.value].sort((a, b) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  )
})
</script>

<template>
  <div class="h-[calc(100vh-12rem)]">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Messages</h1>
      <button
        @click="loadConversations"
        class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
      >
        <RefreshCw class="h-4 w-4" />
      </button>
    </div>

    <div v-if="error" class="mb-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div class="flex h-full bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
      <!-- Conversations List -->
      <div
        :class="[
          'w-full md:w-80 border-r border-gray-200 dark:border-gray-700 flex flex-col',
          selectedConversation ? 'hidden md:flex' : 'flex'
        ]"
      >
        <div class="p-4 border-b border-gray-200 dark:border-gray-700">
          <h2 class="font-medium text-gray-900 dark:text-white">Conversations</h2>
        </div>

        <div v-if="loading && !selectedConversation" class="p-4 text-gray-500">
          Loading...
        </div>

        <div v-else class="flex-1 overflow-y-auto">
          <button
            v-for="conv in conversations"
            :key="conv.phone_number"
            @click="loadMessages(conv.phone_number)"
            :class="[
              'w-full p-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 border-b border-gray-100 dark:border-gray-700',
              selectedConversation === conv.phone_number && 'bg-primary/5'
            ]"
          >
            <div class="flex items-center justify-between">
              <span class="font-medium text-gray-900 dark:text-white">
                {{ formatPhoneNumber(conv.phone_number) }}
              </span>
              <span class="text-xs text-gray-500">
                {{ formatTime(conv.last_message_time) }}
              </span>
            </div>
            <div class="flex items-center justify-between mt-1">
              <p class="text-sm text-gray-500 truncate max-w-[200px]">
                {{ conv.last_message }}
              </p>
              <span
                v-if="conv.unread_count > 0"
                class="inline-flex items-center justify-center w-5 h-5 text-xs font-medium text-white bg-primary rounded-full"
              >
                {{ conv.unread_count }}
              </span>
            </div>
          </button>

          <div v-if="conversations.length === 0" class="p-4 text-center text-gray-500">
            No conversations yet
          </div>
        </div>
      </div>

      <!-- Messages View -->
      <div
        :class="[
          'flex-1 flex flex-col',
          !selectedConversation ? 'hidden md:flex' : 'flex'
        ]"
      >
        <div v-if="selectedConversation" class="flex flex-col h-full">
          <!-- Header -->
          <div class="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center">
            <button
              @click="selectedConversation = null"
              class="md:hidden mr-3 p-1 text-gray-500 hover:text-gray-700"
            >
              <ArrowLeft class="h-5 w-5" />
            </button>
            <MessageSquare class="h-5 w-5 text-gray-400 mr-2" />
            <span class="font-medium text-gray-900 dark:text-white">
              {{ formatPhoneNumber(selectedConversation) }}
            </span>
          </div>

          <!-- Messages -->
          <div class="flex-1 overflow-y-auto p-4 space-y-4">
            <div v-if="loading" class="text-center text-gray-500">Loading...</div>

            <div
              v-for="message in sortedMessages"
              :key="message.id"
              :class="[
                'flex',
                message.direction === 'outbound' ? 'justify-end' : 'justify-start'
              ]"
            >
              <div
                :class="[
                  'max-w-[70%] rounded-lg px-4 py-2',
                  message.direction === 'outbound'
                    ? 'bg-primary text-white'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white'
                ]"
              >
                <p class="whitespace-pre-wrap">{{ message.body }}</p>
                <div
                  v-if="message.media_urls?.length"
                  class="mt-2 space-y-2"
                >
                  <img
                    v-for="(url, index) in message.media_urls"
                    :key="index"
                    :src="url"
                    class="max-w-full rounded"
                    alt="Media attachment"
                  />
                </div>
                <p
                  :class="[
                    'text-xs mt-1',
                    message.direction === 'outbound' ? 'text-white/70' : 'text-gray-500'
                  ]"
                >
                  {{ formatTime(message.created_at) }}
                </p>
              </div>
            </div>
          </div>

          <!-- Input -->
          <div class="p-4 border-t border-gray-200 dark:border-gray-700">
            <form @submit.prevent="sendMessage" class="flex space-x-2">
              <select
                v-model="selectedDID"
                class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white text-sm"
              >
                <option v-for="did in dids" :key="did.id" :value="did.phone_number">
                  {{ did.friendly_name || formatPhoneNumber(did.phone_number) }}
                </option>
              </select>
              <input
                v-model="newMessage"
                type="text"
                placeholder="Type a message..."
                class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
              <button
                type="submit"
                :disabled="sendingMessage || !newMessage.trim()"
                class="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                <Send class="h-4 w-4" />
              </button>
            </form>
          </div>
        </div>

        <!-- Empty State -->
        <div v-else class="flex-1 flex items-center justify-center text-gray-500">
          <div class="text-center">
            <MessageSquare class="h-12 w-12 mx-auto mb-4 text-gray-300" />
            <p>Select a conversation to view messages</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
</script>

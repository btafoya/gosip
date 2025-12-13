<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { systemApi } from '@/api/auth'
import { Phone, MessageSquare, Voicemail, Monitor, AlertCircle, CheckCircle } from 'lucide-vue-next'

interface SystemStatus {
  status: string
  version: string
  uptime: string
  sip_server_status: string
  twilio_status: string
  active_calls: number
  registered_devices: number
  stats: Record<string, number>
}

const status = ref<SystemStatus | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    status.value = await systemApi.getStatus()
  } catch {
    error.value = 'Failed to load system status'
  } finally {
    loading.value = false
  }
})

function formatUptime(uptime: string): string {
  // Parse Go duration format
  const match = uptime.match(/(\d+)h(\d+)m(\d+)/)
  if (match) {
    return `${match[1]}h ${match[2]}m`
  }
  return uptime
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Dashboard</h1>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <div v-else-if="error" class="mt-6 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-else-if="status" class="mt-6 space-y-6">
      <!-- System Status -->
      <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">System Status</h2>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="flex items-center space-x-3">
            <component
              :is="status.status === 'healthy' ? CheckCircle : AlertCircle"
              :class="status.status === 'healthy' ? 'text-green-500' : 'text-yellow-500'"
              class="h-8 w-8"
            />
            <div>
              <p class="text-sm text-gray-500 dark:text-gray-400">Overall Status</p>
              <p class="font-medium capitalize">{{ status.status }}</p>
            </div>
          </div>

          <div class="flex items-center space-x-3">
            <component
              :is="status.sip_server_status === 'online' ? CheckCircle : AlertCircle"
              :class="status.sip_server_status === 'online' ? 'text-green-500' : 'text-red-500'"
              class="h-8 w-8"
            />
            <div>
              <p class="text-sm text-gray-500 dark:text-gray-400">SIP Server</p>
              <p class="font-medium capitalize">{{ status.sip_server_status }}</p>
            </div>
          </div>

          <div class="flex items-center space-x-3">
            <component
              :is="status.twilio_status === 'healthy' ? CheckCircle : AlertCircle"
              :class="status.twilio_status === 'healthy' ? 'text-green-500' : status.twilio_status === 'degraded' ? 'text-yellow-500' : 'text-gray-400'"
              class="h-8 w-8"
            />
            <div>
              <p class="text-sm text-gray-500 dark:text-gray-400">Twilio</p>
              <p class="font-medium capitalize">{{ status.twilio_status }}</p>
            </div>
          </div>
        </div>

        <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 flex justify-between text-sm text-gray-500">
          <span>Version: {{ status.version }}</span>
          <span>Uptime: {{ formatUptime(status.uptime) }}</span>
        </div>
      </div>

      <!-- Quick Stats -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <div class="flex items-center">
            <div class="p-3 rounded-full bg-blue-100 dark:bg-blue-900">
              <Monitor class="h-6 w-6 text-blue-600 dark:text-blue-400" />
            </div>
            <div class="ml-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Registered Devices</p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ status.registered_devices }}
              </p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <div class="flex items-center">
            <div class="p-3 rounded-full bg-green-100 dark:bg-green-900">
              <Phone class="h-6 w-6 text-green-600 dark:text-green-400" />
            </div>
            <div class="ml-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Active Calls</p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ status.active_calls }}
              </p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <div class="flex items-center">
            <div class="p-3 rounded-full bg-purple-100 dark:bg-purple-900">
              <MessageSquare class="h-6 w-6 text-purple-600 dark:text-purple-400" />
            </div>
            <div class="ml-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Total Messages</p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">-</p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <div class="flex items-center">
            <div class="p-3 rounded-full bg-orange-100 dark:bg-orange-900">
              <Voicemail class="h-6 w-6 text-orange-600 dark:text-orange-400" />
            </div>
            <div class="ml-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Unread Voicemails</p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">-</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Stats Summary -->
      <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">System Statistics</h2>
        <dl class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="px-4 py-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <dt class="text-sm text-gray-500 dark:text-gray-400">Total Devices</dt>
            <dd class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ status.stats.total_devices || 0 }}
            </dd>
          </div>
          <div class="px-4 py-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <dt class="text-sm text-gray-500 dark:text-gray-400">Total DIDs</dt>
            <dd class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ status.stats.total_dids || 0 }}
            </dd>
          </div>
          <div class="px-4 py-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <dt class="text-sm text-gray-500 dark:text-gray-400">Total Users</dt>
            <dd class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
              {{ status.stats.total_users || 0 }}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  </div>
</template>

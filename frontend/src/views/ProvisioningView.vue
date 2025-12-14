<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { devicesApi, type Device } from '@/api/devices'
import { provisioningApi, type ProvisioningProfile, type ProvisioningToken, type DeviceEvent } from '@/api/provisioning'
import {
  Smartphone,
  RefreshCw,
  Link,
  Copy,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  Activity,
  Key,
  Trash2,
  Plus,
  ChevronRight,
  Settings2,
  FileText
} from 'lucide-vue-next'

// State
const loading = ref(true)
const error = ref<string | null>(null)
const devices = ref<Device[]>([])
const profiles = ref<ProvisioningProfile[]>([])
const vendors = ref<string[]>([])
const tokens = ref<ProvisioningToken[]>([])
const recentEvents = ref<DeviceEvent[]>([])

// Wizard state
const wizardStep = ref(0)
const selectedDevice = ref<Device | null>(null)
const selectedProfile = ref<ProvisioningProfile | null>(null)
const provisioningResult = ref<{ config_url?: string; token?: string; expires_at?: string; instructions: string } | null>(null)
const provisioning = ref(false)

// Token creation
const showTokenModal = ref(false)
const tokenForm = ref({
  device_id: 0,
  expires_hours: 24,
  max_uses: 1,
  allowed_ip: ''
})
const creatingToken = ref(false)

// Load data
onMounted(async () => {
  await loadData()
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [devicesRes, profilesRes, vendorsRes, tokensRes, eventsRes] = await Promise.all([
      devicesApi.list(),
      provisioningApi.listProfiles(),
      provisioningApi.listVendors(),
      provisioningApi.listTokens({ active_only: true }),
      provisioningApi.getRecentEvents({ limit: 20 })
    ])
    // Devices returns PaginatedResponse with data property
    devices.value = Array.isArray(devicesRes) ? devicesRes : devicesRes?.data || []
    // Profiles returns array directly
    profiles.value = Array.isArray(profilesRes) ? profilesRes : []
    // Vendors returns string array directly
    vendors.value = Array.isArray(vendorsRes) ? vendorsRes : []
    // Tokens returns array directly (or null if empty)
    tokens.value = Array.isArray(tokensRes) ? tokensRes : []
    // Events returns array directly (or null if empty)
    recentEvents.value = Array.isArray(eventsRes) ? eventsRes : []
  } catch (err) {
    error.value = 'Failed to load provisioning data'
    console.error(err)
  } finally {
    loading.value = false
  }
}

// Wizard functions
function startWizard(device: Device) {
  selectedDevice.value = device
  selectedProfile.value = profiles.value.find(p =>
    p.vendor === device.vendor && (!device.model || p.model === device.model)
  ) || profiles.value.find(p => p.vendor === device.vendor && p.is_default) || null
  wizardStep.value = 1
  provisioningResult.value = null
}

function selectProfile(profile: ProvisioningProfile) {
  selectedProfile.value = profile
}

async function provisionDevice() {
  if (!selectedDevice.value) return

  provisioning.value = true
  error.value = null

  try {
    // First update device vendor/model if profile is selected
    if (selectedProfile.value) {
      await devicesApi.update(selectedDevice.value.id, {
        vendor: selectedProfile.value.vendor,
        model: selectedProfile.value.model || undefined
      })
    }

    // Create a provisioning token for the device (24 hours = 86400 seconds)
    const result = await provisioningApi.createToken({
      device_id: selectedDevice.value.id,
      expires_in: 86400,
      max_uses: 5
    })

    // Map result to expected format
    provisioningResult.value = {
      config_url: result.provisioning_url,
      token: result.token.token,
      expires_at: result.token.expires_at,
      instructions: `Configure your ${selectedProfile.value?.vendor || 'device'} to use the provisioning URL above, or manually configure:\n\nSIP Server: Your GoSIP server\nUsername: ${selectedDevice.value.extension}\nPassword: Your device password`
    }
    wizardStep.value = 3
    await loadData() // Refresh tokens and events
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Provisioning failed'
  } finally {
    provisioning.value = false
  }
}

function closeWizard() {
  wizardStep.value = 0
  selectedDevice.value = null
  selectedProfile.value = null
  provisioningResult.value = null
}

// Token management
function openTokenModal(device?: Device) {
  tokenForm.value = {
    device_id: device?.id || 0,
    expires_hours: 24,
    max_uses: 1,
    allowed_ip: ''
  }
  showTokenModal.value = true
}

async function createToken() {
  if (!tokenForm.value.device_id) return

  creatingToken.value = true
  error.value = null

  try {
    // Convert hours to seconds for the API
    await provisioningApi.createToken({
      device_id: tokenForm.value.device_id,
      expires_in: tokenForm.value.expires_hours * 3600,
      max_uses: tokenForm.value.max_uses,
      ip_restriction: tokenForm.value.allowed_ip || undefined
    })
    showTokenModal.value = false
    await loadData()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to create token'
  } finally {
    creatingToken.value = false
  }
}

async function revokeToken(token: ProvisioningToken) {
  if (!confirm('Revoke this provisioning token? The device will no longer be able to fetch its configuration.')) return

  try {
    await provisioningApi.revokeToken(token.id)
    await loadData()
  } catch {
    error.value = 'Failed to revoke token'
  }
}

// Utility functions
function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function getEventIcon(eventType: string) {
  switch (eventType) {
    case 'config_fetch': return FileText
    case 'registration': return CheckCircle
    case 'provision_complete': return CheckCircle
    case 'provision_failed': return XCircle
    case 'auth_failed': return AlertTriangle
    case 'error': return XCircle
    default: return Activity
  }
}

function getEventColor(eventType: string) {
  switch (eventType) {
    case 'config_fetch': return 'text-blue-500'
    case 'registration': return 'text-green-500'
    case 'provision_complete': return 'text-green-500'
    case 'provision_failed': return 'text-red-500'
    case 'auth_failed': return 'text-yellow-500'
    case 'error': return 'text-red-500'
    default: return 'text-gray-500'
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case 'provisioned': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
    case 'pending': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
    case 'failed': return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
  }
}

const unprovisionedDevices = computed(() =>
  devices.value.filter(d => d.provisioning_status !== 'provisioned')
)

const activeTokenCount = computed(() =>
  tokens.value.filter(t => !t.revoked && new Date(t.expires_at) > new Date()).length
)
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Device Provisioning</h1>
        <p class="mt-1 text-sm text-gray-500">
          Configure and provision your SIP devices automatically
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadData"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-6 text-gray-500">Loading...</div>

    <template v-else>
      <!-- Stats Cards -->
      <div class="mt-6 grid grid-cols-1 gap-5 sm:grid-cols-3">
        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
          <div class="p-5">
            <div class="flex items-center">
              <div class="flex-shrink-0">
                <Smartphone class="h-6 w-6 text-gray-400" />
              </div>
              <div class="ml-5 w-0 flex-1">
                <dl>
                  <dt class="text-sm font-medium text-gray-500 truncate">Total Devices</dt>
                  <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ devices.length }}</dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
          <div class="p-5">
            <div class="flex items-center">
              <div class="flex-shrink-0">
                <CheckCircle class="h-6 w-6 text-green-400" />
              </div>
              <div class="ml-5 w-0 flex-1">
                <dl>
                  <dt class="text-sm font-medium text-gray-500 truncate">Provisioned</dt>
                  <dd class="text-lg font-medium text-gray-900 dark:text-white">
                    {{ devices.filter(d => d.provisioning_status === 'provisioned').length }}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
          <div class="p-5">
            <div class="flex items-center">
              <div class="flex-shrink-0">
                <Key class="h-6 w-6 text-blue-400" />
              </div>
              <div class="ml-5 w-0 flex-1">
                <dl>
                  <dt class="text-sm font-medium text-gray-500 truncate">Active Tokens</dt>
                  <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ activeTokenCount }}</dd>
                </dl>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Devices Section -->
      <div class="mt-8">
        <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Devices</h2>
        <div class="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Device
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Vendor/Model
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Provisioning Status
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Last Config Fetch
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
                    <Smartphone class="h-5 w-5 text-gray-400 mr-3" />
                    <div>
                      <div class="font-medium text-gray-900 dark:text-white">{{ device.name }}</div>
                      <div class="text-sm text-gray-500">{{ device.username }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                  <span v-if="device.vendor">{{ device.vendor }}</span>
                  <span v-if="device.model" class="text-gray-500"> / {{ device.model }}</span>
                  <span v-if="!device.vendor" class="text-gray-400">Not set</span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <span
                    :class="[
                      'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                      getStatusBadge(device.provisioning_status || 'unknown')
                    ]"
                  >
                    {{ device.provisioning_status || 'unknown' }}
                  </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ device.last_config_fetch ? formatDate(device.last_config_fetch) : 'Never' }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                  <button
                    @click="startWizard(device)"
                    class="text-primary hover:text-primary/80 mr-3"
                    title="Configure provisioning"
                  >
                    <Settings2 class="h-4 w-4" />
                  </button>
                  <button
                    @click="openTokenModal(device)"
                    class="text-blue-600 hover:text-blue-500"
                    title="Create provisioning token"
                  >
                    <Link class="h-4 w-4" />
                  </button>
                </td>
              </tr>
              <tr v-if="devices.length === 0">
                <td colspan="5" class="px-6 py-12 text-center text-gray-500">
                  No devices configured. Add devices from the Devices page first.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Active Tokens Section -->
      <div class="mt-8">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white">Active Provisioning Tokens</h2>
          <button
            @click="openTokenModal()"
            class="flex items-center px-3 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
          >
            <Plus class="h-4 w-4 mr-1" />
            New Token
          </button>
        </div>
        <div class="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Device
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Token
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Expires
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Uses
                </th>
                <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <tr v-for="token in tokens" :key="token.id" class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                  {{ token.device_name || `Device #${token.device_id}` }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <div class="flex items-center">
                    <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                      {{ token.token.substring(0, 16) }}...
                    </code>
                    <button
                      @click="copyToClipboard(`${window.location.origin}/api/provision/${token.token}`)"
                      class="ml-2 text-gray-400 hover:text-gray-600"
                      title="Copy full URL"
                    >
                      <Copy class="h-4 w-4" />
                    </button>
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <div class="flex items-center">
                    <Clock class="h-4 w-4 mr-1" />
                    {{ formatDate(token.expires_at) }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ token.use_count }} / {{ token.max_uses === 0 ? 'unlimited' : token.max_uses }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                  <button
                    @click="revokeToken(token)"
                    class="text-destructive hover:text-destructive/80"
                    title="Revoke token"
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </td>
              </tr>
              <tr v-if="tokens.length === 0">
                <td colspan="5" class="px-6 py-8 text-center text-gray-500">
                  No active provisioning tokens.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Recent Events Section -->
      <div class="mt-8">
        <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Recent Provisioning Events</h2>
        <div class="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
          <ul class="divide-y divide-gray-200 dark:divide-gray-700">
            <li v-for="event in recentEvents" :key="event.id" class="px-6 py-4">
              <div class="flex items-center">
                <component
                  :is="getEventIcon(event.event_type)"
                  :class="['h-5 w-5 mr-3', getEventColor(event.event_type)]"
                />
                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ event.event_type.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()) }}
                  </div>
                  <div class="text-xs text-gray-500">
                    Device #{{ event.device_id }} - {{ formatDate(event.created_at) }}
                    <span v-if="event.ip_address"> from {{ event.ip_address }}</span>
                  </div>
                </div>
              </div>
            </li>
            <li v-if="recentEvents.length === 0" class="px-6 py-8 text-center text-gray-500">
              No recent events.
            </li>
          </ul>
        </div>
      </div>
    </template>

    <!-- Provisioning Wizard Modal -->
    <div v-if="wizardStep > 0" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="closeWizard" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full p-6">
          <!-- Wizard Steps -->
          <div class="mb-6">
            <div class="flex items-center">
              <div :class="['flex items-center justify-center w-8 h-8 rounded-full text-sm font-medium',
                wizardStep >= 1 ? 'bg-primary text-white' : 'bg-gray-200 text-gray-600']">
                1
              </div>
              <div :class="['flex-1 h-1 mx-2', wizardStep >= 2 ? 'bg-primary' : 'bg-gray-200']" />
              <div :class="['flex items-center justify-center w-8 h-8 rounded-full text-sm font-medium',
                wizardStep >= 2 ? 'bg-primary text-white' : 'bg-gray-200 text-gray-600']">
                2
              </div>
              <div :class="['flex-1 h-1 mx-2', wizardStep >= 3 ? 'bg-primary' : 'bg-gray-200']" />
              <div :class="['flex items-center justify-center w-8 h-8 rounded-full text-sm font-medium',
                wizardStep >= 3 ? 'bg-primary text-white' : 'bg-gray-200 text-gray-600']">
                3
              </div>
            </div>
            <div class="flex justify-between mt-2 text-xs text-gray-500">
              <span>Select Profile</span>
              <span>Configure</span>
              <span>Complete</span>
            </div>
          </div>

          <!-- Step 1: Select Profile -->
          <div v-if="wizardStep === 1">
            <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
              Select Configuration Profile
            </h3>
            <p class="text-sm text-gray-500 mb-4">
              Provisioning <strong>{{ selectedDevice?.name }}</strong>
            </p>

            <div class="space-y-2 max-h-64 overflow-y-auto">
              <div
                v-for="profile in profiles"
                :key="profile.id"
                @click="selectProfile(profile)"
                :class="[
                  'p-4 border rounded-lg cursor-pointer',
                  selectedProfile?.id === profile.id
                    ? 'border-primary bg-primary/5'
                    : 'border-gray-200 dark:border-gray-600 hover:border-gray-300'
                ]"
              >
                <div class="flex items-center justify-between">
                  <div>
                    <div class="font-medium text-gray-900 dark:text-white">{{ profile.name }}</div>
                    <div class="text-sm text-gray-500">{{ profile.vendor }} {{ profile.model || '(All Models)' }}</div>
                  </div>
                  <div v-if="profile.is_default" class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                    Default
                  </div>
                </div>
                <div v-if="profile.description" class="mt-2 text-sm text-gray-500">
                  {{ profile.description }}
                </div>
              </div>
              <div v-if="profiles.length === 0" class="text-center text-gray-500 py-8">
                No profiles available. Create a profile in the admin settings.
              </div>
            </div>

            <div class="flex justify-end space-x-3 mt-6">
              <button
                @click="closeWizard"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200"
              >
                Cancel
              </button>
              <button
                @click="wizardStep = 2"
                :disabled="!selectedProfile"
                class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                Next
                <ChevronRight class="h-4 w-4 ml-1" />
              </button>
            </div>
          </div>

          <!-- Step 2: Configure -->
          <div v-if="wizardStep === 2">
            <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
              Review Configuration
            </h3>

            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 space-y-3">
              <div class="flex justify-between">
                <span class="text-sm text-gray-500">Device:</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedDevice?.name }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-sm text-gray-500">Extension:</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedDevice?.username }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-sm text-gray-500">Profile:</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedProfile?.name }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-sm text-gray-500">Vendor:</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedProfile?.vendor }}</span>
              </div>
            </div>

            <div class="mt-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
              <h4 class="text-sm font-medium text-blue-800 dark:text-blue-200 mb-2">What happens next:</h4>
              <ul class="text-sm text-blue-700 dark:text-blue-300 space-y-1">
                <li>1. A unique provisioning URL will be generated</li>
                <li>2. Configure your device to use this URL</li>
                <li>3. The device will automatically fetch its configuration</li>
                <li>4. SIP registration will happen automatically</li>
              </ul>
            </div>

            <div class="flex justify-between mt-6">
              <button
                @click="wizardStep = 1"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200"
              >
                Back
              </button>
              <button
                @click="provisionDevice"
                :disabled="provisioning"
                class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                {{ provisioning ? 'Provisioning...' : 'Provision Device' }}
              </button>
            </div>
          </div>

          <!-- Step 3: Complete -->
          <div v-if="wizardStep === 3">
            <div class="text-center mb-6">
              <CheckCircle class="h-12 w-12 text-green-500 mx-auto" />
              <h3 class="text-lg font-medium text-gray-900 dark:text-white mt-4">
                Provisioning Ready!
              </h3>
            </div>

            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Provisioning URL
                </label>
                <div class="flex items-center">
                  <code class="flex-1 text-xs bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-600 overflow-x-auto">
                    {{ provisioningResult?.config_url }}
                  </code>
                  <button
                    @click="copyToClipboard(provisioningResult?.config_url || '')"
                    class="ml-2 p-2 text-gray-400 hover:text-gray-600"
                    title="Copy URL"
                  >
                    <Copy class="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div v-if="provisioningResult?.expires_at">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Token Expires
                </label>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  {{ formatDate(provisioningResult.expires_at) }}
                </p>
              </div>
            </div>

            <div class="mt-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
              <h4 class="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">Instructions:</h4>
              <div class="text-sm text-yellow-700 dark:text-yellow-300 whitespace-pre-line">
                {{ provisioningResult?.instructions }}
              </div>
            </div>

            <div class="flex justify-end mt-6">
              <button
                @click="closeWizard"
                class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Token Modal -->
    <div v-if="showTokenModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showTokenModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Create Provisioning Token
          </h3>

          <form @submit.prevent="createToken" class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Device
              </label>
              <select
                v-model="tokenForm.device_id"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              >
                <option value="0" disabled>Select a device</option>
                <option v-for="device in devices" :key="device.id" :value="device.id">
                  {{ device.name }} ({{ device.username }})
                </option>
              </select>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Expires In (hours)
              </label>
              <input
                v-model.number="tokenForm.expires_hours"
                type="number"
                min="1"
                max="720"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Max Uses (0 = unlimited)
              </label>
              <input
                v-model.number="tokenForm.max_uses"
                type="number"
                min="0"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Allowed IP (optional)
              </label>
              <input
                v-model="tokenForm.allowed_ip"
                type="text"
                placeholder="Leave blank to allow any IP"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div class="flex justify-end space-x-3 pt-4">
              <button
                type="button"
                @click="showTokenModal = false"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="creatingToken || !tokenForm.device_id"
                class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                {{ creatingToken ? 'Creating...' : 'Create Token' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

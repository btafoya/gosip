<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { Settings, User, Bell, Shield, Key, Save, AlertCircle, CheckCircle } from 'lucide-vue-next'
import api from '@/api/client'

const authStore = useAuthStore()

const activeTab = ref('profile')
const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

// Profile form
const profileForm = ref({
  email: '',
  name: ''
})

// Password form
const passwordForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: ''
})

// Notification settings
const notificationSettings = ref({
  email_voicemail: true,
  email_missed_call: false,
  email_sms: false,
  push_voicemail: true,
  push_missed_call: true,
  push_sms: true
})

// System settings (admin only)
const systemSettings = ref({
  twilio_account_sid: '',
  twilio_auth_token: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_user: '',
  smtp_password: '',
  gotify_url: '',
  gotify_token: '',
  voicemail_greeting: '',
  timezone: 'America/New_York'
})

const tabs = [
  { id: 'profile', name: 'Profile', icon: User },
  { id: 'password', name: 'Password', icon: Key },
  { id: 'notifications', name: 'Notifications', icon: Bell },
  ...(authStore.isAdmin ? [{ id: 'system', name: 'System', icon: Settings }] : [])
]

onMounted(async () => {
  if (authStore.user) {
    profileForm.value.email = authStore.user.email
    profileForm.value.name = authStore.user.name || ''
  }

  await loadNotificationSettings()

  if (authStore.isAdmin) {
    await loadSystemSettings()
  }
})

async function loadNotificationSettings() {
  try {
    const response = await api.get('/users/me/notifications')
    notificationSettings.value = response.data.data || notificationSettings.value
  } catch {
    console.error('Failed to load notification settings')
  }
}

async function loadSystemSettings() {
  try {
    const response = await api.get('/system/config')
    const config = response.data || {}
    systemSettings.value = {
      twilio_account_sid: config.twilio_account_sid || '',
      twilio_auth_token: '',
      smtp_host: config.smtp_host || '',
      smtp_port: config.smtp_port || 587,
      smtp_user: config.smtp_user || '',
      smtp_password: '',
      gotify_url: config.gotify_url || '',
      gotify_token: '',
      voicemail_greeting: config.voicemail_greeting || '',
      timezone: config.timezone || 'America/New_York'
    }
  } catch {
    console.error('Failed to load system settings')
  }
}

async function saveProfile() {
  saving.value = true
  error.value = null
  success.value = null

  try {
    await api.put('/users/me', profileForm.value)
    success.value = 'Profile updated successfully'
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to update profile'
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    error.value = 'Passwords do not match'
    return
  }

  if (passwordForm.value.new_password.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }

  saving.value = true
  error.value = null
  success.value = null

  try {
    const result = await authStore.changePassword(
      passwordForm.value.current_password,
      passwordForm.value.new_password
    )

    if (result) {
      success.value = 'Password changed successfully'
      passwordForm.value = { current_password: '', new_password: '', confirm_password: '' }
    } else {
      error.value = authStore.error || 'Failed to change password'
    }
  } finally {
    saving.value = false
  }
}

async function saveNotifications() {
  saving.value = true
  error.value = null
  success.value = null

  try {
    await api.put('/users/me/notifications', notificationSettings.value)
    success.value = 'Notification settings updated'
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to update settings'
  } finally {
    saving.value = false
  }
}

async function saveSystemSettings() {
  saving.value = true
  error.value = null
  success.value = null

  try {
    await api.put('/system/config', systemSettings.value)
    success.value = 'System settings updated'
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to update settings'
  } finally {
    saving.value = false
  }
}

function clearMessages() {
  error.value = null
  success.value = null
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Settings</h1>

    <div class="mt-6 flex flex-col md:flex-row gap-6">
      <!-- Tabs -->
      <div class="w-full md:w-48 flex md:flex-col gap-2">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          @click="activeTab = tab.id; clearMessages()"
          :class="[
            'flex items-center px-4 py-2 text-sm font-medium rounded-md',
            activeTab === tab.id
              ? 'bg-primary text-white'
              : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
          ]"
        >
          <component :is="tab.icon" class="h-4 w-4 mr-2" />
          {{ tab.name }}
        </button>
      </div>

      <!-- Content -->
      <div class="flex-1 bg-white dark:bg-gray-800 shadow rounded-lg p-6">
        <!-- Messages -->
        <div v-if="error" class="mb-4 flex items-center bg-destructive/10 text-destructive px-4 py-3 rounded-md">
          <AlertCircle class="h-4 w-4 mr-2" />
          {{ error }}
        </div>
        <div v-if="success" class="mb-4 flex items-center bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 px-4 py-3 rounded-md">
          <CheckCircle class="h-4 w-4 mr-2" />
          {{ success }}
        </div>

        <!-- Profile Tab -->
        <div v-if="activeTab === 'profile'">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Profile Settings</h2>
          <form @submit.prevent="saveProfile" class="space-y-4 max-w-md">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Email Address
              </label>
              <input
                v-model="profileForm.email"
                type="email"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Display Name
              </label>
              <input
                v-model="profileForm.name"
                type="text"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>
            <button
              type="submit"
              :disabled="saving"
              class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
            >
              <Save class="h-4 w-4 mr-2" />
              {{ saving ? 'Saving...' : 'Save Changes' }}
            </button>
          </form>
        </div>

        <!-- Password Tab -->
        <div v-if="activeTab === 'password'">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Change Password</h2>
          <form @submit.prevent="changePassword" class="space-y-4 max-w-md">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Current Password
              </label>
              <input
                v-model="passwordForm.current_password"
                type="password"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                New Password
              </label>
              <input
                v-model="passwordForm.new_password"
                type="password"
                required
                minlength="8"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Confirm New Password
              </label>
              <input
                v-model="passwordForm.confirm_password"
                type="password"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
            </div>
            <button
              type="submit"
              :disabled="saving"
              class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
            >
              <Shield class="h-4 w-4 mr-2" />
              {{ saving ? 'Changing...' : 'Change Password' }}
            </button>
          </form>
        </div>

        <!-- Notifications Tab -->
        <div v-if="activeTab === 'notifications'">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Notification Preferences</h2>
          <form @submit.prevent="saveNotifications" class="space-y-6 max-w-md">
            <div>
              <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-3">Email Notifications</h3>
              <div class="space-y-3">
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.email_voicemail"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">New voicemail</span>
                </label>
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.email_missed_call"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Missed call</span>
                </label>
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.email_sms"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">New SMS/MMS</span>
                </label>
              </div>
            </div>

            <div>
              <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-3">Push Notifications</h3>
              <div class="space-y-3">
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.push_voicemail"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">New voicemail</span>
                </label>
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.push_missed_call"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Missed call</span>
                </label>
                <label class="flex items-center">
                  <input
                    v-model="notificationSettings.push_sms"
                    type="checkbox"
                    class="h-4 w-4 text-primary border-gray-300 rounded focus:ring-primary"
                  />
                  <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">New SMS/MMS</span>
                </label>
              </div>
            </div>

            <button
              type="submit"
              :disabled="saving"
              class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
            >
              <Save class="h-4 w-4 mr-2" />
              {{ saving ? 'Saving...' : 'Save Preferences' }}
            </button>
          </form>
        </div>

        <!-- System Tab (Admin only) -->
        <div v-if="activeTab === 'system' && authStore.isAdmin">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">System Settings</h2>
          <form @submit.prevent="saveSystemSettings" class="space-y-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <!-- Twilio Settings -->
              <div class="space-y-4">
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">Twilio</h3>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Account SID
                  </label>
                  <input
                    v-model="systemSettings.twilio_account_sid"
                    type="text"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Auth Token (leave blank to keep current)
                  </label>
                  <input
                    v-model="systemSettings.twilio_auth_token"
                    type="password"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
              </div>

              <!-- SMTP Settings -->
              <div class="space-y-4">
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">Email (SMTP)</h3>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    SMTP Host
                  </label>
                  <input
                    v-model="systemSettings.smtp_host"
                    type="text"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    SMTP Port
                  </label>
                  <input
                    v-model.number="systemSettings.smtp_port"
                    type="number"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    SMTP Username
                  </label>
                  <input
                    v-model="systemSettings.smtp_user"
                    type="text"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    SMTP Password
                  </label>
                  <input
                    v-model="systemSettings.smtp_password"
                    type="password"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
              </div>

              <!-- Gotify Settings -->
              <div class="space-y-4">
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">Push Notifications (Gotify)</h3>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Gotify URL
                  </label>
                  <input
                    v-model="systemSettings.gotify_url"
                    type="url"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Gotify Token
                  </label>
                  <input
                    v-model="systemSettings.gotify_token"
                    type="password"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  />
                </div>
              </div>

              <!-- General Settings -->
              <div class="space-y-4">
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">General</h3>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Timezone
                  </label>
                  <select
                    v-model="systemSettings.timezone"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                  >
                    <option value="America/New_York">Eastern Time</option>
                    <option value="America/Chicago">Central Time</option>
                    <option value="America/Denver">Mountain Time</option>
                    <option value="America/Los_Angeles">Pacific Time</option>
                    <option value="UTC">UTC</option>
                  </select>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    Voicemail Greeting Text
                  </label>
                  <textarea
                    v-model="systemSettings.voicemail_greeting"
                    rows="3"
                    class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                    placeholder="Please leave a message after the tone."
                  />
                </div>
              </div>
            </div>

            <button
              type="submit"
              :disabled="saving"
              class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
            >
              <Save class="h-4 w-4 mr-2" />
              {{ saving ? 'Saving...' : 'Save System Settings' }}
            </button>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

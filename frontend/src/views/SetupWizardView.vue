<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { setupApi, type SetupRequest } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const step = ref(1)
const loading = ref(false)
const error = ref<string | null>(null)

const form = ref<SetupRequest>({
  twilio_account_sid: '',
  twilio_auth_token: '',
  admin_email: '',
  admin_password: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_user: '',
  smtp_password: '',
  gotify_url: '',
  gotify_token: ''
})

const confirmPassword = ref('')

function nextStep() {
  if (step.value === 1) {
    if (!form.value.twilio_account_sid || !form.value.twilio_auth_token) {
      error.value = 'Twilio credentials are required'
      return
    }
  }
  if (step.value === 2) {
    if (!form.value.admin_email || !form.value.admin_password) {
      error.value = 'Admin email and password are required'
      return
    }
    if (form.value.admin_password !== confirmPassword.value) {
      error.value = 'Passwords do not match'
      return
    }
    if (form.value.admin_password.length < 8) {
      error.value = 'Password must be at least 8 characters'
      return
    }
  }
  error.value = null
  step.value++
}

function prevStep() {
  error.value = null
  step.value--
}

async function handleSubmit() {
  loading.value = true
  error.value = null

  try {
    await setupApi.complete(form.value)
    authStore.setupCompleted = true
    await authStore.login(form.value.admin_email, form.value.admin_password)
    router.push('/')
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Setup failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 py-12 px-4 sm:px-6 lg:px-8">
    <div class="max-w-lg w-full space-y-8">
      <div>
        <h1 class="text-center text-3xl font-bold text-primary">GoSIP</h1>
        <h2 class="mt-6 text-center text-2xl font-bold text-gray-900 dark:text-white">
          Setup Wizard
        </h2>
        <p class="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
          Let's configure your SIP phone system
        </p>
      </div>

      <!-- Progress steps -->
      <div class="flex justify-center space-x-4">
        <div
          v-for="s in 4"
          :key="s"
          :class="[
            'w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium',
            s <= step ? 'bg-primary text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-500'
          ]"
        >
          {{ s }}
        </div>
      </div>

      <div v-if="error" class="bg-destructive/10 text-destructive px-4 py-3 rounded-md text-sm">
        {{ error }}
      </div>

      <form @submit.prevent="step === 4 ? handleSubmit() : nextStep()" class="space-y-6">
        <!-- Step 1: Twilio Credentials -->
        <div v-if="step === 1" class="space-y-4">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white">Twilio Configuration</h3>
          <p class="text-sm text-gray-500">Enter your Twilio account credentials</p>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Account SID
            </label>
            <input
              v-model="form.twilio_account_sid"
              type="text"
              required
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Auth Token
            </label>
            <input
              v-model="form.twilio_auth_token"
              type="password"
              required
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="Your Twilio Auth Token"
            />
          </div>
        </div>

        <!-- Step 2: Admin Account -->
        <div v-if="step === 2" class="space-y-4">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white">Admin Account</h3>
          <p class="text-sm text-gray-500">Create your administrator account</p>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Email Address
            </label>
            <input
              v-model="form.admin_email"
              type="email"
              required
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="admin@example.com"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Password
            </label>
            <input
              v-model="form.admin_password"
              type="password"
              required
              minlength="8"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="Minimum 8 characters"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Confirm Password
            </label>
            <input
              v-model="confirmPassword"
              type="password"
              required
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="Confirm your password"
            />
          </div>
        </div>

        <!-- Step 3: Email (Optional) -->
        <div v-if="step === 3" class="space-y-4">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white">Email Notifications (Optional)</h3>
          <p class="text-sm text-gray-500">Configure SMTP for email notifications</p>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              SMTP Host
            </label>
            <input
              v-model="form.smtp_host"
              type="text"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="smtp.example.com"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              SMTP Port
            </label>
            <input
              v-model.number="form.smtp_port"
              type="number"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="587"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              SMTP Username
            </label>
            <input
              v-model="form.smtp_user"
              type="text"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="username"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              SMTP Password
            </label>
            <input
              v-model="form.smtp_password"
              type="password"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="password"
            />
          </div>
        </div>

        <!-- Step 4: Push Notifications (Optional) -->
        <div v-if="step === 4" class="space-y-4">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white">Push Notifications (Optional)</h3>
          <p class="text-sm text-gray-500">Configure Gotify for push notifications</p>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Gotify URL
            </label>
            <input
              v-model="form.gotify_url"
              type="url"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="https://gotify.example.com"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Gotify Token
            </label>
            <input
              v-model="form.gotify_token"
              type="password"
              class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-800 dark:text-white"
              placeholder="Your Gotify app token"
            />
          </div>
        </div>

        <!-- Navigation buttons -->
        <div class="flex justify-between">
          <button
            v-if="step > 1"
            type="button"
            @click="prevStep"
            class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
          >
            Back
          </button>
          <div v-else />

          <button
            type="submit"
            :disabled="loading"
            class="px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90 disabled:opacity-50"
          >
            <span v-if="loading">Setting up...</span>
            <span v-else-if="step === 4">Complete Setup</span>
            <span v-else>Next</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

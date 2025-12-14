<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Users, Plus, Edit2, Trash2, RefreshCw, Shield, User } from 'lucide-vue-next'
import api from '@/api/client'

interface UserRecord {
  id: number
  email: string
  name: string
  role: 'admin' | 'user'
  created_at: string
  last_login?: string
}

const users = ref<UserRecord[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const showModal = ref(false)
const editingUser = ref<UserRecord | null>(null)
const saving = ref(false)

const form = ref({
  email: '',
  name: '',
  password: '',
  role: 'user' as 'admin' | 'user'
})

onMounted(async () => {
  await loadUsers()
})

async function loadUsers() {
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/users')
    users.value = response.data.data || []
  } catch {
    error.value = 'Failed to load users'
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  editingUser.value = null
  form.value = { email: '', name: '', password: '', role: 'user' }
  showModal.value = true
}

function openEditModal(user: UserRecord) {
  editingUser.value = user
  form.value = {
    email: user.email,
    name: user.name || '',
    password: '',
    role: user.role
  }
  showModal.value = true
}

async function handleSubmit() {
  saving.value = true
  error.value = null

  try {
    if (editingUser.value) {
      const payload: Record<string, unknown> = {
        email: form.value.email,
        name: form.value.name,
        role: form.value.role
      }
      if (form.value.password) {
        payload.password = form.value.password
      }
      await api.put(`/users/${editingUser.value.id}`, payload)
    } else {
      await api.post('/users', form.value)
    }
    showModal.value = false
    await loadUsers()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Operation failed'
  } finally {
    saving.value = false
  }
}

async function handleDelete(user: UserRecord) {
  if (!confirm(`Delete user "${user.email}"?\n\nThis action cannot be undone.`)) return

  try {
    await api.delete(`/users/${user.id}`)
    await loadUsers()
  } catch {
    error.value = 'Failed to delete user'
  }
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleString()
}
</script>

<template>
  <div>
    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Users</h1>
        <p class="mt-1 text-sm text-gray-500">
          {{ users.length }} user{{ users.length !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="flex space-x-2">
        <button
          @click="loadUsers"
          class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
        >
          <RefreshCw class="h-4 w-4" />
        </button>
        <button
          @click="openCreateModal"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          <Plus class="h-4 w-4 mr-2" />
          Add User
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
              User
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Role
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Created
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Last Login
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr v-for="user in users" :key="user.id" class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <div class="p-2 rounded-full bg-gray-100 dark:bg-gray-700">
                  <component
                    :is="user.role === 'admin' ? Shield : User"
                    class="h-4 w-4 text-gray-500"
                  />
                </div>
                <div class="ml-3">
                  <p class="font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
                  <p v-if="user.name" class="text-sm text-gray-500">{{ user.name }}</p>
                </div>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                :class="[
                  'inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium capitalize',
                  user.role === 'admin'
                    ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
                ]"
              >
                {{ user.role }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(user.created_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(user.last_login) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <button
                @click="openEditModal(user)"
                class="text-primary hover:text-primary/80 mr-3"
              >
                <Edit2 class="h-4 w-4" />
              </button>
              <button
                @click="handleDelete(user)"
                class="text-destructive hover:text-destructive/80"
                :disabled="users.length === 1"
                :class="users.length === 1 && 'opacity-50 cursor-not-allowed'"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-center justify-center min-h-screen px-4">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75" @click="showModal = false" />

        <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
            {{ editingUser ? 'Edit User' : 'Add User' }}
          </h3>

          <form @submit.prevent="handleSubmit" class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Email Address
              </label>
              <input
                v-model="form.email"
                type="email"
                required
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="user@example.com"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Display Name
              </label>
              <input
                v-model="form.name"
                type="text"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="John Doe"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Password {{ editingUser ? '(leave blank to keep current)' : '' }}
              </label>
              <input
                v-model="form.password"
                type="password"
                :required="!editingUser"
                minlength="8"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
                placeholder="Minimum 8 characters"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Role
              </label>
              <select
                v-model="form.role"
                class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              >
                <option value="user">User</option>
                <option value="admin">Admin</option>
              </select>
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
                {{ saving ? 'Saving...' : (editingUser ? 'Update' : 'Create') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, setupApi, type User } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('token'))
  const initialized = ref(false)
  const setupCompleted = ref(true)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function checkAuth() {
    if (initialized.value) return

    try {
      // First check if setup is completed (public endpoint)
      const status = await setupApi.getStatus()
      setupCompleted.value = status.setup_completed

      if (!status.setup_completed) {
        initialized.value = true
        return
      }

      // Then check if user is authenticated
      if (token.value) {
        try {
          const currentUser = await authApi.getCurrentUser()
          user.value = currentUser
        } catch {
          // Token invalid or expired
          user.value = null
          token.value = null
          localStorage.removeItem('token')
        }
      }
    } catch {
      // Setup status check failed - assume setup not completed
      setupCompleted.value = false
    } finally {
      initialized.value = true
    }
  }

  async function login(email: string, password: string) {
    loading.value = true
    error.value = null

    try {
      const response = await authApi.login({ email, password })
      user.value = response.user
      token.value = response.token
      localStorage.setItem('token', response.token)
      return true
    } catch (err: unknown) {
      const apiError = err as { response?: { data?: { error?: { message?: string } } } }
      error.value = apiError.response?.data?.error?.message || 'Login failed'
      return false
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      user.value = null
      token.value = null
      localStorage.removeItem('token')
    }
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    loading.value = true
    error.value = null

    try {
      await authApi.changePassword(currentPassword, newPassword)
      return true
    } catch (err: unknown) {
      const apiError = err as { response?: { data?: { error?: { message?: string } } } }
      error.value = apiError.response?.data?.error?.message || 'Password change failed'
      return false
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    user,
    token,
    initialized,
    setupCompleted,
    loading,
    error,
    isAuthenticated,
    isAdmin,
    checkAuth,
    login,
    logout,
    changePassword,
    clearError
  }
})

<script setup lang="ts">
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  Phone,
  MessageSquare,
  Voicemail,
  Settings,
  Users,
  Monitor,
  Route,
  LayoutDashboard,
  LogOut,
  Menu,
  X
} from 'lucide-vue-next'
import { ref } from 'vue'

const route = useRoute()
const authStore = useAuthStore()
const sidebarOpen = ref(false)

const navigation = [
  { name: 'Dashboard', href: '/', icon: LayoutDashboard },
  { name: 'Devices', href: '/devices', icon: Monitor },
  { name: 'Phone Numbers', href: '/dids', icon: Phone },
  { name: 'Call Routing', href: '/routes', icon: Route },
  { name: 'Call History', href: '/calls', icon: Phone },
  { name: 'Messages', href: '/messages', icon: MessageSquare },
  { name: 'Voicemails', href: '/voicemails', icon: Voicemail },
  { name: 'Settings', href: '/settings', icon: Settings }
]

const adminNavigation = [
  { name: 'Users', href: '/users', icon: Users }
]

function isActive(href: string) {
  if (href === '/') {
    return route.path === '/'
  }
  return route.path.startsWith(href)
}

async function handleLogout() {
  await authStore.logout()
}
</script>

<template>
  <div class="min-h-screen bg-gray-100 dark:bg-gray-900">
    <!-- Mobile sidebar -->
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 lg:hidden">
      <div class="fixed inset-0 bg-gray-600 bg-opacity-75" @click="sidebarOpen = false" />
      <div class="fixed inset-y-0 left-0 flex w-64 flex-col bg-white dark:bg-gray-800">
        <div class="flex h-16 items-center justify-between px-4">
          <span class="text-xl font-bold text-primary">GoSIP</span>
          <button @click="sidebarOpen = false" class="text-gray-500">
            <X class="h-6 w-6" />
          </button>
        </div>
        <nav class="flex-1 space-y-1 px-2 py-4">
          <RouterLink
            v-for="item in navigation"
            :key="item.name"
            :to="item.href"
            :class="[
              isActive(item.href)
                ? 'bg-primary/10 text-primary'
                : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
              'group flex items-center px-3 py-2 text-sm font-medium rounded-md'
            ]"
            @click="sidebarOpen = false"
          >
            <component
              :is="item.icon"
              :class="[
                isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                'mr-3 h-5 w-5 flex-shrink-0'
              ]"
            />
            {{ item.name }}
          </RouterLink>

          <template v-if="authStore.isAdmin">
            <div class="pt-4 pb-2">
              <p class="px-3 text-xs font-semibold text-gray-400 uppercase">Admin</p>
            </div>
            <RouterLink
              v-for="item in adminNavigation"
              :key="item.name"
              :to="item.href"
              :class="[
                isActive(item.href)
                  ? 'bg-primary/10 text-primary'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
                'group flex items-center px-3 py-2 text-sm font-medium rounded-md'
              ]"
              @click="sidebarOpen = false"
            >
              <component
                :is="item.icon"
                :class="[
                  isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                  'mr-3 h-5 w-5 flex-shrink-0'
                ]"
              />
              {{ item.name }}
            </RouterLink>
          </template>
        </nav>
      </div>
    </div>

    <!-- Desktop sidebar -->
    <div class="hidden lg:fixed lg:inset-y-0 lg:flex lg:w-64 lg:flex-col">
      <div class="flex min-h-0 flex-1 flex-col border-r border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
        <div class="flex h-16 items-center px-4 border-b border-gray-200 dark:border-gray-700">
          <span class="text-xl font-bold text-primary">GoSIP</span>
        </div>
        <nav class="flex-1 space-y-1 px-2 py-4">
          <RouterLink
            v-for="item in navigation"
            :key="item.name"
            :to="item.href"
            :class="[
              isActive(item.href)
                ? 'bg-primary/10 text-primary'
                : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
              'group flex items-center px-3 py-2 text-sm font-medium rounded-md'
            ]"
          >
            <component
              :is="item.icon"
              :class="[
                isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                'mr-3 h-5 w-5 flex-shrink-0'
              ]"
            />
            {{ item.name }}
          </RouterLink>

          <template v-if="authStore.isAdmin">
            <div class="pt-4 pb-2">
              <p class="px-3 text-xs font-semibold text-gray-400 uppercase">Admin</p>
            </div>
            <RouterLink
              v-for="item in adminNavigation"
              :key="item.name"
              :to="item.href"
              :class="[
                isActive(item.href)
                  ? 'bg-primary/10 text-primary'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
                'group flex items-center px-3 py-2 text-sm font-medium rounded-md'
              ]"
            >
              <component
                :is="item.icon"
                :class="[
                  isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                  'mr-3 h-5 w-5 flex-shrink-0'
                ]"
              />
              {{ item.name }}
            </RouterLink>
          </template>
        </nav>

        <div class="border-t border-gray-200 dark:border-gray-700 p-4">
          <div class="flex items-center">
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300 truncate">
                {{ authStore.user?.email }}
              </p>
              <p class="text-xs text-gray-500 truncate">
                {{ authStore.user?.role }}
              </p>
            </div>
            <button
              @click="handleLogout"
              class="ml-3 p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              title="Sign out"
            >
              <LogOut class="h-5 w-5" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Main content -->
    <div class="lg:pl-64">
      <!-- Top bar -->
      <div class="sticky top-0 z-10 flex h-16 flex-shrink-0 bg-white dark:bg-gray-800 shadow lg:hidden">
        <button
          @click="sidebarOpen = true"
          class="px-4 text-gray-500 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary"
        >
          <Menu class="h-6 w-6" />
        </button>
        <div class="flex flex-1 items-center justify-center">
          <span class="text-xl font-bold text-primary">GoSIP</span>
        </div>
        <div class="w-14" /> <!-- Spacer for symmetry -->
      </div>

      <main class="py-6">
        <div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <RouterView />
        </div>
      </main>
    </div>
  </div>
</template>

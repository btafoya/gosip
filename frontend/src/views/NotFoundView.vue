<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Home, ArrowLeft } from 'lucide-vue-next'

const router = useRouter()

function goBack() {
  router.back()
}

function goHome() {
  router.push('/')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 py-12 px-4">
    <div class="max-w-md w-full text-center">
      <h1 class="text-9xl font-bold text-primary">404</h1>
      <h2 class="mt-4 text-2xl font-bold text-gray-900 dark:text-white">
        Page not found
      </h2>
      <p class="mt-2 text-gray-500">
        Sorry, we couldn't find the page you're looking for.
      </p>

      <div class="mt-8 flex justify-center space-x-4">
        <button
          @click="goBack"
          class="flex items-center px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600"
        >
          <ArrowLeft class="h-4 w-4 mr-2" />
          Go back
        </button>
        <button
          @click="goHome"
          class="flex items-center px-4 py-2 text-sm font-medium text-white bg-primary rounded-md hover:bg-primary/90"
        >
          <Home class="h-4 w-4 mr-2" />
          Go home
        </button>
      </div>
    </div>
  </div>
</template>

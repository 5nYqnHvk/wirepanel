<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Network, LogIn, Loader2 } from 'lucide-vue-next'

const auth = useAuthStore()
const router = useRouter()
const username = ref('admin')
const password = ref('wirepanel')
const loading = ref(false)

async function submit() {
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    router.push('/dashboard')
  } catch {} finally { loading.value = false }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-bg p-6">
    <div class="w-full max-w-sm card p-8 shadow-xl">
      <div class="flex items-center gap-3 mb-6">
        <div class="w-10 h-10 rounded-lg bg-accent/15 flex items-center justify-center">
          <Network class="w-5 h-5 text-accent" />
        </div>
        <div>
          <h1 class="text-lg font-semibold">WirePanel</h1>
          <p class="text-xs text-muted">Linux infrastructure panel</p>
        </div>
      </div>

      <form @submit.prevent="submit" class="space-y-3">
        <label class="block">
          <span class="text-xs text-muted">Username</span>
          <input v-model="username" class="input w-full mt-1" autofocus />
        </label>
        <label class="block">
          <span class="text-xs text-muted">Password</span>
          <input v-model="password" type="password" class="input w-full mt-1" />
        </label>
        <p v-if="auth.error" class="text-sm text-danger">{{ auth.error }}</p>
        <button class="btn-primary w-full" :disabled="loading">
          <Loader2 v-if="loading" class="w-4 h-4 animate-spin" />
          <LogIn v-else class="w-4 h-4" />
          Sign in
        </button>
      </form>
    </div>
  </div>
</template>

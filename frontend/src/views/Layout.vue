<script setup lang="ts">
import { RouterView, RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { LayoutDashboard, Server, LogOut, Network, ScrollText, ShieldCheck } from 'lucide-vue-next'

const auth = useAuthStore()
const router = useRouter()

function logout() {
  auth.logout()
  router.push('/login')
}

function roleBadge(role: string | null) {
  if (role === 'admin') return 'badge bg-danger/10 text-danger border-danger/30'
  if (role === 'operator') return 'badge bg-warn/10 text-warn border-warn/30'
  return 'badge bg-accent/10 text-accent border-accent/30'
}
</script>

<template>
  <div class="min-h-screen flex bg-bg">
    <aside class="w-60 shrink-0 border-r border-border bg-surface flex flex-col">
      <div class="p-4 border-b border-border flex items-center gap-2">
        <div class="w-8 h-8 rounded-md bg-accent/15 flex items-center justify-center">
          <Network class="w-4 h-4 text-accent" />
        </div>
        <span class="font-semibold tracking-tight">WirePanel</span>
      </div>

      <div class="px-4 py-3 border-b border-border">
        <p class="text-xs text-muted">signed in as</p>
        <p class="font-mono text-sm">{{ auth.user || '-' }}</p>
        <span :class="roleBadge(auth.role)" class="mt-1"><ShieldCheck class="w-3 h-3" /> {{ auth.role }}</span>
      </div>

      <nav class="flex-1 p-3 space-y-1 text-sm">
        <RouterLink to="/dashboard" class="flex items-center gap-2 px-3 py-2 rounded-md row-hover" active-class="bg-panel text-accent">
          <LayoutDashboard class="w-4 h-4" /> Dashboard
        </RouterLink>
        <RouterLink to="/agents" class="flex items-center gap-2 px-3 py-2 rounded-md row-hover" active-class="bg-panel text-accent">
          <Server class="w-4 h-4" /> Agents
        </RouterLink>
        <RouterLink v-if="auth.can('audit.read')" to="/audit" class="flex items-center gap-2 px-3 py-2 rounded-md row-hover" active-class="bg-panel text-accent">
          <ScrollText class="w-4 h-4" /> Audit
        </RouterLink>
      </nav>
      <button @click="logout" class="m-3 btn-ghost"><LogOut class="w-4 h-4" /> Logout</button>
    </aside>
    <main class="flex-1 min-w-0 overflow-auto">
      <RouterView />
    </main>
  </div>
</template>

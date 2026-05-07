<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Agent } from '@/api/client'
import { Server, Activity, Cpu, Wifi, WifiOff } from 'lucide-vue-next'

const agents = ref<Agent[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try { agents.value = await api.agents() } finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold">Dashboard</h1>
        <p class="text-sm text-muted">Overview of registered nodes</p>
      </div>
      <button @click="load" class="btn-ghost"><Activity class="w-4 h-4" /> Refresh</button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="card p-4">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-md bg-accent/15 flex items-center justify-center"><Server class="w-5 h-5 text-accent" /></div>
          <div>
            <p class="text-xs text-muted">Total agents</p>
            <p class="text-2xl font-semibold">{{ agents.length }}</p>
          </div>
        </div>
      </div>
      <div class="card p-4">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-md bg-ok/15 flex items-center justify-center"><Wifi class="w-5 h-5 text-ok" /></div>
          <div>
            <p class="text-xs text-muted">Online</p>
            <p class="text-2xl font-semibold">{{ agents.filter(a => a.online).length }}</p>
          </div>
        </div>
      </div>
      <div class="card p-4">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-md bg-danger/15 flex items-center justify-center"><WifiOff class="w-5 h-5 text-danger" /></div>
          <div>
            <p class="text-xs text-muted">Offline</p>
            <p class="text-2xl font-semibold">{{ agents.filter(a => !a.online).length }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="px-4 py-3 border-b border-border flex items-center gap-2">
        <Cpu class="w-4 h-4 text-muted" />
        <span class="font-medium">Recent agents</span>
      </div>
      <div v-if="loading" class="p-6 text-sm text-muted">Loading...</div>
      <div v-else-if="!agents.length" class="p-6 text-sm text-muted">No agents registered. Run the agent binary to connect.</div>
      <ul v-else class="divide-y divide-border">
        <li v-for="a in agents" :key="a.id" class="px-4 py-3 row-hover">
          <RouterLink :to="`/agents/${a.id}`" class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium truncate">{{ a.hostname }}</p>
              <p class="text-xs text-muted truncate">{{ a.os }}/{{ a.arch }} · v{{ a.version }}</p>
            </div>
            <span :class="a.online ? 'badge-ok' : 'badge-off'">{{ a.online ? 'online' : 'offline' }}</span>
          </RouterLink>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Agent } from '@/api/client'
import { RefreshCw } from 'lucide-vue-next'

const agents = ref<Agent[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try { agents.value = await api.agents() } finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="p-6 space-y-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-semibold">Agents</h1>
      <button @click="load" class="btn-ghost"><RefreshCw class="w-4 h-4" /> Refresh</button>
    </div>

    <div class="card overflow-hidden">
      <table class="w-full text-sm">
        <thead class="text-muted text-xs uppercase tracking-wide">
          <tr class="border-b border-border">
            <th class="text-left px-4 py-3">Hostname</th>
            <th class="text-left px-4 py-3">ID</th>
            <th class="text-left px-4 py-3">OS / Arch</th>
            <th class="text-left px-4 py-3">Version</th>
            <th class="text-left px-4 py-3">Last seen</th>
            <th class="text-left px-4 py-3">Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading"><td colspan="6" class="p-6 text-muted">Loading...</td></tr>
          <tr v-else-if="!agents.length"><td colspan="6" class="p-6 text-muted">No agents</td></tr>
          <tr v-for="a in agents" :key="a.id" class="border-b border-border row-hover">
            <td class="px-4 py-3">
              <RouterLink :to="`/agents/${a.id}`" class="text-accent hover:underline">{{ a.hostname }}</RouterLink>
            </td>
            <td class="px-4 py-3 font-mono text-xs text-muted">{{ a.id }}</td>
            <td class="px-4 py-3">{{ a.os }}/{{ a.arch }}</td>
            <td class="px-4 py-3">{{ a.version }}</td>
            <td class="px-4 py-3 text-muted">{{ new Date(a.last_seen).toLocaleString() }}</td>
            <td class="px-4 py-3"><span :class="a.online ? 'badge-ok' : 'badge-off'">{{ a.online ? 'online' : 'offline' }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type SystemInfo } from '@/api/client'
import { Cpu, MemoryStick, Server, Clock, RefreshCw } from 'lucide-vue-next'

const props = defineProps<{ agentId: string }>()
const info = ref<SystemInfo | null>(null)
const loading = ref(false)
const error = ref('')

function fmtUptime(s: number): string {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return `${d}d ${h}h ${m}m`
}
function fmtKB(kb: number): string {
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(0)} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}

async function load() {
  loading.value = true
  error.value = ''
  try { info.value = await api.system(props.agentId) }
  catch (e: any) { error.value = e?.response?.data?.error || 'failed' }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end">
      <button @click="load" class="btn-ghost"><RefreshCw class="w-4 h-4" :class="loading && 'animate-spin'" /> Refresh</button>
    </div>

    <div v-if="error" class="card p-4 text-danger">{{ error }}</div>

    <div v-if="info" class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div class="card p-5">
        <div class="flex items-center gap-2 text-muted text-xs uppercase tracking-wide mb-3"><Server class="w-4 h-4" /> System</div>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-muted">Hostname</dt><dd>{{ info.hostname }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Distro</dt><dd>{{ info.distro || '-' }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">OS / Arch</dt><dd>{{ info.os }}/{{ info.arch }}</dd></div>
          <div class="flex justify-between gap-4"><dt class="text-muted shrink-0">Kernel</dt><dd class="text-right text-xs font-mono truncate">{{ info.kernel || '-' }}</dd></div>
        </dl>
      </div>

      <div class="card p-5">
        <div class="flex items-center gap-2 text-muted text-xs uppercase tracking-wide mb-3"><Cpu class="w-4 h-4" /> CPU & load</div>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-muted">Cores</dt><dd>{{ info.cpu_count }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Load 1m</dt><dd>{{ info.load_avg_1.toFixed(2) }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Load 5m</dt><dd>{{ info.load_avg_5.toFixed(2) }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Load 15m</dt><dd>{{ info.load_avg_15.toFixed(2) }}</dd></div>
        </dl>
      </div>

      <div class="card p-5">
        <div class="flex items-center gap-2 text-muted text-xs uppercase tracking-wide mb-3"><MemoryStick class="w-4 h-4" /> Memory</div>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between"><dt class="text-muted">Total</dt><dd>{{ fmtKB(info.mem_total_kb) }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Available</dt><dd>{{ fmtKB(info.mem_avail_kb) }}</dd></div>
          <div class="flex justify-between"><dt class="text-muted">Free</dt><dd>{{ fmtKB(info.mem_free_kb) }}</dd></div>
        </dl>
        <div class="mt-3 h-2 rounded bg-panel overflow-hidden">
          <div class="h-full bg-accent" :style="{width: `${100 - (info.mem_avail_kb/info.mem_total_kb*100)}%`}"></div>
        </div>
      </div>

      <div class="card p-5">
        <div class="flex items-center gap-2 text-muted text-xs uppercase tracking-wide mb-3"><Clock class="w-4 h-4" /> Uptime</div>
        <p class="text-3xl font-mono">{{ fmtUptime(info.uptime_sec) }}</p>
        <p class="text-xs text-muted mt-1">since boot</p>
      </div>
    </div>
  </div>
</template>

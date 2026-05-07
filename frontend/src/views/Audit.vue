<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type AuditEntry } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { RefreshCw, Undo2, ShieldCheck, ShieldAlert, Eye, X } from 'lucide-vue-next'
import ConfirmModal from '@/components/ConfirmModal.vue'

const auth = useAuthStore()
const entries = ref<AuditEntry[]>([])
const loading = ref(false)
const detail = ref<AuditEntry | null>(null)

const rbTarget = ref<AuditEntry | null>(null)
const rbBusy = ref(false)

async function load() {
  loading.value = true
  try { entries.value = await api.auditList() } finally { loading.value = false }
}

async function rollback() {
  if (!rbTarget.value) return
  rbBusy.value = true
  try {
    await api.auditRollback(rbTarget.value.id)
    rbTarget.value = null
    await load()
  } catch (e: any) { alert(e?.response?.data?.error || e?.response?.data || 'rollback failed') }
  finally { rbBusy.value = false }
}

function statusClass(s: string) {
  if (s === 'ok') return 'badge-ok'
  if (s === 'rolled_back') return 'badge bg-warn/10 text-warn border-warn/30'
  return 'badge bg-danger/10 text-danger border-danger/30'
}

onMounted(load)
</script>

<template>
  <div class="p-6 space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold">Audit log</h1>
        <p class="text-sm text-muted">Append-only record of dangerous actions. Reversible entries can be rolled back.</p>
      </div>
      <button @click="load" class="btn-ghost"><RefreshCw class="w-4 h-4" :class="loading && 'animate-spin'" /> Refresh</button>
    </div>

    <div class="card overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="text-muted text-xs uppercase">
          <tr class="border-b border-border">
            <th class="text-left px-4 py-2">When</th>
            <th class="text-left px-4 py-2">User</th>
            <th class="text-left px-4 py-2">Agent</th>
            <th class="text-left px-4 py-2">Action</th>
            <th class="text-left px-4 py-2">Target</th>
            <th class="text-left px-4 py-2">Status</th>
            <th class="text-left px-4 py-2">Reversible</th>
            <th class="px-4 py-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading"><td colspan="8" class="p-4 text-muted">Loading...</td></tr>
          <tr v-else-if="!entries.length"><td colspan="8" class="p-4 text-muted">no audit entries</td></tr>
          <tr v-for="e in entries" :key="e.id" class="border-b border-border row-hover">
            <td class="px-4 py-2 text-xs text-muted">{{ new Date(e.timestamp).toLocaleString() }}</td>
            <td class="px-4 py-2"><span class="font-mono">{{ e.user }}</span> <span class="text-xs text-muted">({{ e.role }})</span></td>
            <td class="px-4 py-2 font-mono text-xs">{{ e.agent_id || '-' }}</td>
            <td class="px-4 py-2 font-mono">{{ e.action }}</td>
            <td class="px-4 py-2 font-mono text-xs truncate max-w-[20ch]">{{ e.target || '-' }}</td>
            <td class="px-4 py-2"><span :class="statusClass(e.status)">{{ e.status }}</span></td>
            <td class="px-4 py-2">
              <span v-if="e.reversible && e.status !== 'rolled_back'" class="badge-ok"><ShieldCheck class="w-3 h-3" /> yes</span>
              <span v-else class="badge-off"><ShieldAlert class="w-3 h-3" /> no</span>
            </td>
            <td class="px-4 py-2 text-right">
              <div class="inline-flex gap-1">
                <button @click="detail = e" class="btn-ghost text-xs" title="details"><Eye class="w-3 h-3" /></button>
                <button v-if="auth.can('audit.rollback') && e.reversible && e.status !== 'rolled_back'"
                  @click="rbTarget = e" class="btn-danger text-xs" title="rollback">
                  <Undo2 class="w-3 h-3" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="detail" class="fixed inset-0 bg-black/70 z-40 flex items-center justify-center p-6" @click.self="detail = null">
      <div class="card w-full max-w-2xl p-5 max-h-[80vh] overflow-auto">
        <div class="flex items-center justify-between mb-4">
          <h3 class="font-semibold">Audit entry</h3>
          <button @click="detail = null" class="btn-ghost"><X class="w-4 h-4" /></button>
        </div>
        <pre class="text-xs font-mono bg-bg p-4 rounded overflow-auto">{{ JSON.stringify(detail, null, 2) }}</pre>
      </div>
    </div>

    <ConfirmModal
      :open="!!rbTarget"
      destructive
      title="Rollback action"
      :message="`This will replay the inverse of action \`${rbTarget?.action}\` on target \`${rbTarget?.target ?? ''}\`. The original change will be undone using the captured pre-image. This is itself an action that may not be reversible.`"
      :confirm-phrase="`rollback ${rbTarget?.id?.slice(0, 8)}`"
      :busy="rbBusy"
      @cancel="rbTarget = null"
      @confirm="rollback"
    />
  </div>
</template>

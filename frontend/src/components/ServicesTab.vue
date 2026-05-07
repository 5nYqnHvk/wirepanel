<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type Service } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { Play, Square, RotateCcw, RefreshCw, Search, Power, PowerOff } from 'lucide-vue-next'
import ConfirmModal from '@/components/ConfirmModal.vue'

const props = defineProps<{ agentId: string }>()
const auth = useAuthStore()
const services = ref<Service[]>([])
const loading = ref(false)
const filter = ref('')
const error = ref('')

const pending = ref<{ svc: Service; action: string } | null>(null)
const busy = ref(false)

const filtered = computed(() =>
  services.value.filter(s => !filter.value || s.name.toLowerCase().includes(filter.value.toLowerCase()))
)

async function load() {
  loading.value = true
  error.value = ''
  try { services.value = await api.services(props.agentId) }
  catch (e: any) { error.value = e?.response?.data?.error || e?.response?.data || 'failed' }
  finally { loading.value = false }
}

function ask(s: Service, act: string) {
  if (!canAct(act)) { alert('no permission'); return }
  pending.value = { svc: s, action: act }
}

async function doAction() {
  if (!pending.value) return
  busy.value = true
  try {
    await api.serviceAction(props.agentId, pending.value.svc.name, pending.value.action)
    pending.value = null
    await load()
  } catch (e: any) { alert(e?.response?.data?.error || e?.response?.data || 'action failed') }
  finally { busy.value = false }
}

function canAct(action: string): boolean {
  if (action === 'enable' || action === 'disable') return auth.can('services.admin')
  return auth.can('services.state')
}

function activeBadge(s: Service) {
  return s.active === 'active' ? 'badge-ok' : 'badge-off'
}

onMounted(load)
</script>

<template>
  <div class="space-y-3">
    <div class="card p-3 flex items-center gap-2">
      <Search class="w-4 h-4 text-muted ml-1" />
      <input v-model="filter" placeholder="Filter services..." class="input flex-1" />
      <button @click="load" class="btn-ghost"><RefreshCw class="w-4 h-4" :class="loading && 'animate-spin'" /></button>
    </div>

    <div v-if="error" class="card p-4 text-danger">{{ error }}</div>

    <div class="card overflow-hidden">
      <table class="w-full text-sm">
        <thead class="text-muted text-xs uppercase">
          <tr class="border-b border-border">
            <th class="text-left px-4 py-2">Service</th>
            <th class="text-left px-4 py-2">Load</th>
            <th class="text-left px-4 py-2">Active</th>
            <th class="text-left px-4 py-2">Sub</th>
            <th class="px-4 py-2 text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !services.length"><td colspan="5" class="p-4 text-muted">Loading...</td></tr>
          <tr v-else-if="!filtered.length"><td colspan="5" class="p-4 text-muted">no services</td></tr>
          <tr v-for="s in filtered" :key="s.name" class="border-b border-border row-hover">
            <td class="px-4 py-2">
              <p class="font-mono">{{ s.name }}</p>
              <p class="text-xs text-muted truncate max-w-md">{{ s.description }}</p>
            </td>
            <td class="px-4 py-2 text-muted">{{ s.load }}</td>
            <td class="px-4 py-2"><span :class="activeBadge(s)">{{ s.active }}</span></td>
            <td class="px-4 py-2 text-muted">{{ s.sub }}</td>
            <td class="px-4 py-2 text-right">
              <div class="inline-flex gap-1">
                <button v-if="auth.can('services.state')" @click="ask(s, 'start')" class="btn-ghost text-xs" title="start"><Play class="w-3 h-3" /></button>
                <button v-if="auth.can('services.state')" @click="ask(s, 'stop')" class="btn-ghost text-xs" title="stop"><Square class="w-3 h-3" /></button>
                <button v-if="auth.can('services.state')" @click="ask(s, 'restart')" class="btn-ghost text-xs" title="restart"><RotateCcw class="w-3 h-3" /></button>
                <button v-if="auth.can('services.admin')" @click="ask(s, 'enable')" class="btn-ghost text-xs" title="enable"><Power class="w-3 h-3" /></button>
                <button v-if="auth.can('services.admin')" @click="ask(s, 'disable')" class="btn-ghost text-xs" title="disable"><PowerOff class="w-3 h-3" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmModal
      :open="!!pending"
      :destructive="pending?.action === 'stop' || pending?.action === 'disable'"
      :title="`${pending?.action ?? ''} service`"
      :message="`Run \`systemctl ${pending?.action} ${pending?.svc.name}\`? Prior state is captured for rollback.`"
      :confirm-phrase="pending?.svc.name ?? ''"
      :busy="busy"
      @cancel="pending = null"
      @confirm="doAction"
    />
  </div>
</template>

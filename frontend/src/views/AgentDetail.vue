<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import OverviewTab from '@/components/OverviewTab.vue'
import TerminalTab from '@/components/TerminalTab.vue'
import FileManagerTab from '@/components/FileManagerTab.vue'
import ServicesTab from '@/components/ServicesTab.vue'
import { useAuthStore } from '@/stores/auth'
import { Activity, Terminal, FolderTree, Cog, ArrowLeft } from 'lucide-vue-next'

const props = defineProps<{ id: string }>()
const auth = useAuthStore()
const agentId = computed(() => props.id)

const tabs = computed(() => {
  const t: { key: string; label: string; icon: any; show: boolean }[] = [
    { key: 'overview', label: 'Overview', icon: Activity,   show: auth.can('system.read') },
    { key: 'terminal', label: 'Terminal', icon: Terminal,   show: auth.can('terminal') },
    { key: 'files',    label: 'Files',    icon: FolderTree, show: auth.can('fs.read') },
    { key: 'services', label: 'Services', icon: Cog,        show: auth.can('services.read') }
  ]
  return t.filter(x => x.show)
})

const active = ref<string>('overview')
</script>

<template>
  <div class="p-6 space-y-4">
    <div class="flex items-center gap-3">
      <RouterLink to="/agents" class="btn-ghost"><ArrowLeft class="w-4 h-4" /> Agents</RouterLink>
      <h1 class="text-xl font-semibold font-mono">{{ agentId }}</h1>
    </div>

    <div class="flex border-b border-border">
      <button v-for="t in tabs" :key="t.key" @click="active = t.key"
        class="flex items-center gap-2 px-4 py-2 text-sm border-b-2 -mb-px transition-colors"
        :class="active === t.key ? 'border-accent text-accent' : 'border-transparent text-muted hover:text-slate-200'">
        <component :is="t.icon" class="w-4 h-4" /> {{ t.label }}
      </button>
    </div>

    <div>
      <OverviewTab v-if="active === 'overview'" :agent-id="agentId" />
      <TerminalTab v-else-if="active === 'terminal'" :agent-id="agentId" />
      <FileManagerTab v-else-if="active === 'files'" :agent-id="agentId" />
      <ServicesTab v-else-if="active === 'services'" :agent-id="agentId" />
    </div>
  </div>
</template>

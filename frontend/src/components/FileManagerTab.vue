<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type FSEntry } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { Folder, FileText, ChevronUp, RefreshCw, Trash2, FolderPlus, Save, X, ShieldAlert } from 'lucide-vue-next'
import ConfirmModal from '@/components/ConfirmModal.vue'

const props = defineProps<{ agentId: string }>()
const auth = useAuthStore()
const path = ref('/')
const entries = ref<FSEntry[]>([])
const loading = ref(false)
const error = ref('')

const editing = ref<{ path: string; content: string } | null>(null)
const newDir = ref('')
const showMkdir = ref(false)

const deleteTarget = ref<FSEntry | null>(null)
const deleteBusy = ref(false)
const writeBusy = ref(false)
const writeConfirm = ref(false)

async function load(p?: string) {
  if (p !== undefined) path.value = p
  loading.value = true
  error.value = ''
  try {
    const r = await api.fsList(props.agentId, path.value)
    entries.value = (r.entries ?? []).sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
      return a.name.localeCompare(b.name)
    })
    path.value = r.path
  } catch (e: any) { error.value = e?.response?.data?.error || e?.response?.data || 'failed' }
  finally { loading.value = false }
}

function up() {
  const p = path.value.replace(/\/+$/, '')
  const parent = p.substring(0, p.lastIndexOf('/')) || '/'
  load(parent)
}

async function open(e: FSEntry) {
  if (e.is_dir) return load(e.path)
  if (!auth.can('fs.read')) { alert('no permission to read files'); return }
  if (e.size > 2 * 1024 * 1024) { alert('file too large to open inline'); return }
  try {
    const r = await api.fsRead(props.agentId, e.path)
    editing.value = { path: r.path, content: r.content }
  } catch (er: any) { alert(er?.response?.data?.error || er?.response?.data || 'read failed') }
}

async function save() {
  if (!editing.value || !auth.can('fs.write')) return
  writeConfirm.value = true
}

async function doSave() {
  if (!editing.value) return
  writeBusy.value = true
  try {
    await api.fsWrite(props.agentId, editing.value.path, editing.value.content)
    writeConfirm.value = false
    editing.value = null
    await load()
  } catch (e: any) { alert(e?.response?.data?.error || e?.response?.data || 'write failed') }
  finally { writeBusy.value = false }
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleteBusy.value = true
  try {
    await api.fsDelete(props.agentId, deleteTarget.value.path, deleteTarget.value.is_dir)
    deleteTarget.value = null
    await load()
  } catch (er: any) { alert(er?.response?.data?.error || er?.response?.data || 'delete failed') }
  finally { deleteBusy.value = false }
}

async function mkdir() {
  if (!newDir.value) return
  const p = path.value.replace(/\/+$/, '') + '/' + newDir.value
  try { await api.fsMkdir(props.agentId, p); newDir.value = ''; showMkdir.value = false; await load() }
  catch (er: any) { alert(er?.response?.data?.error || er?.response?.data || 'mkdir failed') }
}

function fmtSize(b: number): string {
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  if (b < 1024 * 1024 * 1024) return (b / 1024 / 1024).toFixed(1) + ' MB'
  return (b / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

onMounted(() => load())
</script>

<template>
  <div class="space-y-3">
    <div class="card p-3 flex items-center gap-2">
      <button @click="up" class="btn-ghost"><ChevronUp class="w-4 h-4" /></button>
      <input v-model="path" @keyup.enter="load()" class="input flex-1 font-mono" />
      <button @click="load()" class="btn-ghost"><RefreshCw class="w-4 h-4" /></button>
      <button v-if="auth.can('fs.mkdir')" @click="showMkdir = !showMkdir" class="btn-ghost"><FolderPlus class="w-4 h-4" /></button>
    </div>

    <div v-if="showMkdir" class="card p-3 flex items-center gap-2">
      <input v-model="newDir" placeholder="new directory name" class="input flex-1" />
      <button @click="mkdir" class="btn-primary">Create</button>
      <button @click="showMkdir = false" class="btn-ghost"><X class="w-4 h-4" /></button>
    </div>

    <div v-if="error" class="card p-4 text-danger">{{ error }}</div>

    <div class="card overflow-hidden">
      <table class="w-full text-sm">
        <thead class="text-muted text-xs uppercase">
          <tr class="border-b border-border">
            <th class="text-left px-4 py-2">Name</th>
            <th class="text-left px-4 py-2">Size</th>
            <th class="text-left px-4 py-2">Mode</th>
            <th class="text-left px-4 py-2">Modified</th>
            <th class="px-4 py-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading"><td colspan="5" class="p-4 text-muted">Loading...</td></tr>
          <tr v-else-if="!entries.length"><td colspan="5" class="p-4 text-muted">empty</td></tr>
          <tr v-for="e in entries" :key="e.path" class="border-b border-border row-hover">
            <td class="px-4 py-2">
              <button @click="open(e)" class="flex items-center gap-2 text-left">
                <Folder v-if="e.is_dir" class="w-4 h-4 text-accent2" />
                <FileText v-else class="w-4 h-4 text-muted" />
                <span :class="e.is_dir && 'text-accent'">{{ e.name }}</span>
              </button>
            </td>
            <td class="px-4 py-2 text-muted">{{ e.is_dir ? '-' : fmtSize(e.size) }}</td>
            <td class="px-4 py-2 font-mono text-xs text-muted">{{ e.mode }}</td>
            <td class="px-4 py-2 text-muted text-xs">{{ new Date(e.mod_time).toLocaleString() }}</td>
            <td class="px-4 py-2 text-right">
              <button v-if="auth.can('fs.delete')" @click="deleteTarget = e" class="btn-danger text-xs"><Trash2 class="w-3 h-3" /></button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="editing" class="fixed inset-0 bg-black/60 flex items-center justify-center z-40 p-6">
      <div class="card w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div class="px-4 py-2 border-b border-border flex items-center justify-between">
          <span class="font-mono text-sm">{{ editing.path }}</span>
          <div class="flex gap-2">
            <button v-if="auth.can('fs.write')" @click="save" class="btn-primary text-xs"><Save class="w-3 h-3" /> Save</button>
            <button @click="editing = null" class="btn-ghost text-xs"><X class="w-3 h-3" /></button>
          </div>
        </div>
        <textarea v-model="editing.content" class="flex-1 p-4 bg-bg font-mono text-xs resize-none focus:outline-none" spellcheck="false"></textarea>
      </div>
    </div>

    <ConfirmModal
      :open="!!deleteTarget"
      destructive
      title="Delete file"
      :message="deleteTarget?.is_dir
        ? `This will recursively MOVE the directory ${deleteTarget?.path} to the audit trash. Rollback restores it from trash. Trash is emptied manually only.`
        : `This will MOVE ${deleteTarget?.path} to audit trash. Rollback restores it from trash.`"
      :confirm-phrase="deleteTarget?.path ?? ''"
      :busy="deleteBusy"
      @cancel="deleteTarget = null"
      @confirm="doDelete"
    />

    <ConfirmModal
      :open="writeConfirm"
      title="Save file"
      :message="`Overwrite ${editing?.path}? Prior content will be saved as audit pre-image and is rollback-recoverable.`"
      :confirm-phrase="editing?.path ?? ''"
      :busy="writeBusy"
      @cancel="writeConfirm = false"
      @confirm="doSave"
    />
  </div>
</template>

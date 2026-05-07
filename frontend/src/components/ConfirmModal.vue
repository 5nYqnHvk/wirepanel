<script setup lang="ts">
import { ref, watch } from 'vue'
import { AlertTriangle, X } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  title: string
  message: string
  confirmPhrase: string
  destructive?: boolean
  busy?: boolean
}>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'confirm'): void
}>()

const typed = ref('')
watch(() => props.open, (v) => { if (v) typed.value = '' })

const matches = () => typed.value === props.confirmPhrase
</script>

<template>
  <div v-if="open" class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-6" @click.self="!busy && emit('cancel')">
    <div class="card w-full max-w-md p-5">
      <div class="flex items-start gap-3">
        <div class="w-10 h-10 rounded-md flex items-center justify-center"
          :class="destructive ? 'bg-danger/15 text-danger' : 'bg-warn/15 text-warn'">
          <AlertTriangle class="w-5 h-5" />
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="font-semibold text-base">{{ title }}</h3>
          <p class="text-sm text-muted mt-1 whitespace-pre-line">{{ message }}</p>
        </div>
        <button @click="emit('cancel')" class="btn-ghost text-muted" :disabled="busy"><X class="w-4 h-4" /></button>
      </div>

      <div class="mt-4">
        <p class="text-xs text-muted mb-2">
          Type <span class="font-mono text-slate-200">{{ confirmPhrase }}</span> to confirm:
        </p>
        <input v-model="typed" class="input w-full font-mono" autofocus :disabled="busy" />
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button @click="emit('cancel')" class="btn-ghost" :disabled="busy">Cancel</button>
        <button @click="emit('confirm')" :disabled="!matches() || busy"
          :class="destructive ? 'btn-danger' : 'btn-primary'">
          {{ busy ? 'Working...' : 'Confirm' }}
        </button>
      </div>
    </div>
  </div>
</template>

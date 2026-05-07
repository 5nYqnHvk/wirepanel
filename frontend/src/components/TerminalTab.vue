<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { api } from '@/api/client'

const props = defineProps<{ agentId: string }>()
const termEl = ref<HTMLDivElement>()
const status = ref<'connecting' | 'open' | 'closed'>('connecting')

let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObs: ResizeObserver | null = null

function b64encode(s: string): string {
  return btoa(unescape(encodeURIComponent(s)))
}
function b64decode(s: string): string {
  return decodeURIComponent(escape(atob(s)))
}

onMounted(() => {
  if (!termEl.value) return
  term = new Terminal({
    fontFamily: 'JetBrains Mono, ui-monospace, monospace',
    fontSize: 13,
    theme: {
      background: '#0b0f14',
      foreground: '#e2e8f0',
      cursor: '#3ad6c0'
    },
    cursorBlink: true,
    convertEol: true
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(termEl.value)
  fit.fit()

  ws = new WebSocket(api.terminalWS(props.agentId))
  ws.onopen = () => {
    status.value = 'open'
    sendResize()
  }
  ws.onclose = () => { status.value = 'closed'; term?.write('\r\n[connection closed]\r\n') }
  ws.onerror = () => { status.value = 'closed' }
  ws.onmessage = (ev) => {
    try {
      const env = JSON.parse(ev.data)
      if (env.type === 'term.output' && env.payload?.data) {
        term?.write(b64decode(env.payload.data))
      } else if (env.type === 'term.close') {
        term?.write('\r\n[session closed]\r\n')
      }
    } catch {}
  }

  term.onData((d) => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data: b64encode(d) }))
    }
  })
  term.onResize(() => sendResize())

  resizeObs = new ResizeObserver(() => fit?.fit())
  resizeObs.observe(termEl.value)
})

function sendResize() {
  if (!ws || ws.readyState !== WebSocket.OPEN || !term) return
  ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
}

onBeforeUnmount(() => {
  resizeObs?.disconnect()
  ws?.close()
  term?.dispose()
})
</script>

<template>
  <div class="card overflow-hidden">
    <div class="px-4 py-2 border-b border-border flex items-center justify-between text-xs text-muted">
      <span>web terminal · {{ agentId }}</span>
      <span :class="status === 'open' ? 'badge-ok' : 'badge-off'">{{ status }}</span>
    </div>
    <div ref="termEl" class="h-[600px] bg-bg p-2"></div>
  </div>
</template>

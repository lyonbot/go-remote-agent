<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, type PropType } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { useAgentStore } from '@/stores/agent'
import type { PtyTermOptions } from '@/services/pty.service'
import { debounce } from 'lodash-es'

const props = defineProps({
  options: { type: Object as PropType<PtyTermOptions>, required: true },
})

const terminalRef = ref(null)

const term = new Terminal()

const agentStore = useAgentStore()
const ptyService = computed(() => agentStore.ptyService!) // assume ptyService is ready

onMounted(() => {
  term.open(terminalRef.value!)
  term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m\r\n')

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  fitAddon.fit()

  term.focus()
  term.onData(data => { ptyService.value.sendTermData(data) })
  window.addEventListener('resize', debounce(() => {
    fitAddon.fit()
    const dem = fitAddon.proposeDimensions()!
    ptyService.value.resizeTerm(dem.cols, dem.rows)
  }, 500))

  ptyService.value.setTerm(term)
  ptyService.value.connect().then(() => {
    ptyService.value.createPty(props.options)
  })
})

onUnmounted(() => {
  ptyService.value.setTerm(null)
  term.dispose()
})
</script>

<template>
  <div ref="terminalRef" class="terminal-container"></div>
</template>

<style>
.terminal-container {
  width: 100%;
  height: 100%;
}
</style>
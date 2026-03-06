<script setup lang="ts">
import { computed, ref, watch, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { useAgentStore } from '@/stores/agent'
import TerminalContainer from '@/components/TerminalContainer.vue'
import FileTransferToolbar from '@/components/FileTransferToolbar.vue'

const route = useRoute()
const agentStore = useAgentStore()

const agentId = computed(() => Number(route.params.agentId))
const isConnecting = ref(true)
watch(agentId, (id) => {
  agentStore.selectedAgentId = id
  agentStore.reloadAgentInstances().then(() => {
    const id = agentId.value
    const agent = agentStore.agentInstances.find(a => a.id === id)
    if (!agent) return
    if (agentStore.ptyService?.agentId !== id) {
      agentStore.connectPtyService(agent)
      isConnecting.value = true
    }
    agentStore.ptyService?.connect().then(() => {
      isConnecting.value = false
    })
  })
}, { immediate: true })
</script>

<template>
  <div class="flex flex-col flex-1 min-h-0 gap-2">
    <TerminalContainer class="flex-1 min-h-0" />
    <FileTransferToolbar />
  </div>
</template>

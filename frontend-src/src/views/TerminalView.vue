<script setup lang="ts">
import { computed, watch, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { useAgentStore } from '@/stores/agent'
import TerminalContainer from '@/components/TerminalContainer.vue'
import FileTransferToolbar from '@/components/FileTransferToolbar.vue'

const route = useRoute()
const agentStore = useAgentStore()
agentStore.reloadAgentInstances()

const agentId = computed(() => Number(route.params.agentId))
watchEffect(() => {
  agentStore.selectedAgentId = agentId.value
})
</script>

<template>
  <div class="flex flex-col flex-1 min-h-0 gap-2">
    <TerminalContainer class="flex-1 min-h-0" />
    <FileTransferToolbar />
  </div>
</template>

<script setup lang="ts">
import { useAgentStore } from '@/stores/agent'
import AgentList from './components/AgentList.vue'
import TerminalContainer from './components/TerminalContainer.vue'
import FileTransferToolbar from './components/FileTransferToolbar.vue'
import ProxyManager from './components/ProxyManager.vue'

const agentStore = useAgentStore()
</script>

<template>
  <div class="app-container">
    <template v-if="!agentStore.ptyService">
      <AgentList style="flex: 1" />

      <ProxyManager />
    </template>

    <template v-else>
      <TerminalContainer />
      <FileTransferToolbar />
    </template>
  </div>
</template>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  padding: 16px;
  gap: 16px;
}
</style>
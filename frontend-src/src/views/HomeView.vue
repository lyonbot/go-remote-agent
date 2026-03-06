<script setup lang="ts">
import { useAgentStore } from '@/stores/agent'
import AgentList from '@/components/AgentList.vue'

const agentStore = useAgentStore()
</script>

<template>
  <AgentList
    v-model:selectedAgentId="agentStore.selectedAgentId"
    :selectedAgent="agentStore.selectedAgent"
    @connect="agentStore.connectPtyService($event)"
  />
</template>

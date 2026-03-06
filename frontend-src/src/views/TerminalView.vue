<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAgentStore } from '@/stores/agent'
import TerminalContainer from '@/components/TerminalContainer.vue'
import FileTransferToolbar from '@/components/FileTransferToolbar.vue'
import FileSystemContainer from '@/components/FileSystemContainer.vue'

const route = useRoute()
const agentStore = useAgentStore()

const agentId = computed(() => Number(route.params.agentId))
const isConnecting = ref(true)
const activeTab = ref<'terminal' | 'files'>('terminal')

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
    <!-- Tabs -->
    <div class="flex items-center gap-1 shrink-0 border-b border-border pb-1">
      <button @click="activeTab = 'terminal'"
        class="btn-sm"
        :class="activeTab === 'terminal' ? 'btn-primary' : 'btn-ghost'">
        Terminal
      </button>
      <button @click="activeTab = 'files'"
        class="btn-sm"
        :class="activeTab === 'files' ? 'btn-primary' : 'btn-ghost'">
        Files
      </button>
    </div>
    <!-- Tab content -->
    <div class="flex-1 min-h-0 relative">
      <TerminalContainer v-show="activeTab === 'terminal'" class="absolute inset-0 contain-layout" />
      <FileSystemContainer v-show="activeTab === 'files'" class="absolute inset-0 contain-layout" />
    </div>
    <FileTransferToolbar class="shrink-0" />
  </div>
</template>

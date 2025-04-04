<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAgentStore } from '@/stores/agent'
import { UpgradeService } from '@/services/upgrade.service'

const agentStore = useAgentStore()

const showDialog = ref(false)
const logs = ref('')

async function startUpgrade() {
  const { agentInstance } = agentStore
  if (!agentInstance) return

  const upgradeService = new UpgradeService(agentStore.apiKey)
  showDialog.value = true
  logs.value = `agent: ${agentInstance.name}\n`
  logs.value += `agent_id: ${agentInstance.id}\n\n`

  await upgradeService.startUpgrade(
    agentInstance.name,
    agentInstance.id,
    (log) => logs.value += log
  )
}

const isUpgradable = computed(() => !!agentStore.agentInstance?.is_upgradable)
</script>

<template>
  <button @click="startUpgrade" v-if="isUpgradable">升级Agent</button>
  <div v-if="showDialog" class="upgrade-dialog">
    <div class="dialog-content">
      <h2>升级日志</h2>
      <pre class="logs">{{ logs }}</pre>
      <div class="dialog-actions">
        <button @click="showDialog = false">关闭</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.upgrade-dialog {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.dialog-content {
  background-color: white;
  padding: 20px;
  border-radius: 4px;
  max-width: 80%;
  max-height: 80%;
  overflow: auto;
}

.logs {
  background-color: #f5f5f5;
  padding: 10px;
  border-radius: 4px;
  max-height: 400px;
  overflow: auto;
  white-space: pre-wrap;
  font-family: monospace;
}

.dialog-actions {
  margin-top: 20px;
  text-align: right;
}
</style>
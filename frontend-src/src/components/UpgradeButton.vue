<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAgentStore } from '@/stores/agent'
import type { AgentDef } from '@/stores/agent'
import { UpgradeService } from '@/services/upgrade.service'

const props = defineProps<{ agentInstance?: AgentDef }>()

const agentStore = useAgentStore()

const showDialog = ref(false)
const logs = ref('')

async function startUpgrade() {
  const { agentInstance } = props
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

const isUpgradable = computed(() => !!props.agentInstance?.is_upgradable)
</script>

<template>
  <button v-if="isUpgradable" @click="startUpgrade" class="btn btn-warning shrink-0">升级</button>

  <Teleport to="body">
    <div
      v-if="showDialog"
      class="fixed inset-0 bg-black/70 flex items-center justify-center z-50"
      @click.self="showDialog = false"
    >
      <div class="panel p-5 w-full max-w-2xl max-h-[80vh] flex flex-col shadow-2xl">
        <h2 class="section-title mb-3 shrink-0">升级日志</h2>
        <pre class="flex-1 overflow-auto bg-base border border-border rounded p-3 text-xs font-mono text-fg-dim whitespace-pre-wrap min-h-0">{{ logs }}</pre>
        <div class="flex justify-end mt-3 shrink-0">
          <button @click="showDialog = false" class="btn btn-ghost">关闭</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

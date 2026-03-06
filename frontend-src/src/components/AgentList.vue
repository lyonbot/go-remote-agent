<template>
  <div class="flex flex-col h-full">
    <form class="flex gap-2 mb-3 shrink-0 items-center" @submit.prevent="agentStore.reloadAgentInstances()" aria-label="API 配置">
      <label for="api-key-field" class="sr-only">API Key</label>
      <input
        id="api-key-field"
        type="password"
        v-model="configStore.apiKey"
        placeholder="API Key"
        autocomplete="current-password"
        class="field flex-1"
      >
      <button type="submit" class="btn btn-ghost">Fetch</button>
      <button
        type="button"
        @click="handleConnect"
        :disabled="!props.selectedAgent || !!agentStore.ptyService"
        :aria-disabled="!props.selectedAgent || !!agentStore.ptyService"
        class="btn btn-primary"
      >Connect</button>
      <UpgradeButton :agentInstance="props.selectedAgent" />
    </form>

    <ul class="flex flex-col gap-2 overflow-y-auto flex-1 min-h-0" role="listbox" aria-label="Agent 列表">
      <li
        v-for="agent in agentStore.agentInstances"
        :key="agent.id"
        role="option"
        :aria-selected="agent.id === props.selectedAgentId"
        :class="[
          'p-3 rounded-lg border cursor-pointer transition-colors outline-none',
          agent.id === props.selectedAgentId
            ? 'border-primary bg-raised'
            : 'border-border hover:bg-raised/60 hover:border-border-strong'
        ]"
        @click="emit('update:selectedAgentId', agent.id)"
        @dblclick="emit('connect', agent)"
        @keydown.enter="emit('update:selectedAgentId', agent.id)"
        @keydown.space.prevent="emit('update:selectedAgentId', agent.id)"
        tabindex="0"
      >
        <div class="flex gap-2 items-center mb-1.5">
          <span class="font-semibold text-fg text-sm">{{ agent.name }}</span>
          <span class="text-xs font-mono text-fg-muted bg-control/60 px-1.5 py-0.5 rounded" :aria-label="`Agent ID: ${agent.id}`">
            #{{ agent.id }}
          </span>
          <span v-if="agent.is_upgradable" role="status" class="text-xs bg-warning text-white px-1.5 py-0.5 rounded">
            可升级
          </span>
        </div>
        <div class="flex flex-col gap-0.5">
          <div class="flex gap-3 text-xs text-fg-muted font-mono">
            <span>{{ new Date(agent.join_at).toLocaleString() }}</span>
            <span>{{ agent.remote_addr }}</span>
          </div>
          <div class="text-xs text-fg-subtle truncate font-mono" :title="agent.user_agent">
            {{ agent.user_agent }}
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { useAgentStore } from '../stores/agent'
import type { AgentDef } from '../stores/agent'
import { useConfigStore } from '../stores/config'
import UpgradeButton from './UpgradeButton.vue'

const props = defineProps<{
  selectedAgentId: number
  selectedAgent?: AgentDef
}>()

const emit = defineEmits<{
  'update:selectedAgentId': [id: number]
  'connect': [agent: AgentDef]
}>()

const agentStore = useAgentStore()
const configStore = useConfigStore()

agentStore.reloadAgentInstances()

function handleConnect() {
  if (props.selectedAgent) {
    emit('connect', props.selectedAgent)
  }
}
</script>

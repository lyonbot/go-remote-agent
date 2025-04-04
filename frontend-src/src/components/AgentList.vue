<template>
  <div class="agent-list">
    <form class="api-key-input" @submit.prevent="agentStore.reloadAgentInstances()">
      <input type="password" v-model="agentStore.apiKey" placeholder="Enter API Key">
      <button @click="agentStore.reloadAgentInstances()">Fetch List</button>
      <button @click="agentStore.connectPtyService()" :disabled="!!agentStore.ptyService">Connect Pty</button>
      <UpgradeButton />
    </form>

    <div class="agents">
      <div v-for="agent in agentStore.agentInstances" :key="agent.id" class="agent-item"
        :class="{ active: agent.id === agentStore.agentId }" @click="agentStore.agentId = agent.id"
        @dblclick="agentStore.connectPtyService()">
        <div class="agent-header">
          <div class="agent-name">{{ agent.name }}</div>
          <div class="agent-id">id: {{ agent.id }}</div>
          <div class="agent-badge" v-if="agent.is_upgradable">可升级</div>
        </div>
        <div class="agent-details">
          <div class="agent-info">
            <span class="join-time">加入时间：{{ new Date(agent.join_at).toLocaleString() }}</span>
            <span class="remote-addr">IP：{{ agent.remote_addr }}</span>
          </div>
          <div class="agent-ua">{{ agent.user_agent }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAgentStore } from '../stores/agent'
import UpgradeButton from './UpgradeButton.vue';

const agentStore = useAgentStore()

agentStore.reloadAgentInstances()
</script>

<style scoped>
.agent-list {
  display: flex;
  flex-direction: column;
}

.api-key-input {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-shrink: 0;
}

.api-key-input input {
  flex: 1;
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
}

.api-key-input button {
  padding: 0.5rem 1rem;
  background: #4CAF50;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.api-key-input button:hover {
  background: #45a049;
}

.agents {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1 0 0;
  overflow-y: auto;
}

.agent-item {
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}

.agent-item:hover {
  background: #f5f5f5;
}

.agent-item.active {
  border-color: #4CAF50;
  background: #e8f5e9;
}

.agent-name {
  font-weight: bold;
}

.agent-id {
  font-size: 0.8em;
  color: #888;
  padding: 4px 8px;
  border-radius: 4px;
  background: #f5f5f5;
}

.agent-header {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.5rem;
}

.agent-badge {
  background: #ff9800;
  color: white;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8em;
}

.agent-details {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.agent-ua {
  font-size: 0.85em;
  color: #888;
  word-break: break-all;
}

.agent-info,
.join-time,
.remote-addr {
  display: block;
  font-size: 0.9em;
  color: #666;
}
</style>
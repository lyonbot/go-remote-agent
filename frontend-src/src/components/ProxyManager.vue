<script lang="ts" setup>
import { effect, onMounted, ref, watch } from 'vue'
import { useAgentStore } from '@/stores/agent'
import { type ProxyDef, ProxyService } from '@/services/proxy.service'
const agentStore = useAgentStore()

const proxyService = new ProxyService(agentStore.apiKey)
effect(() => { proxyService.apiKey = agentStore.apiKey })

const proxyList = ref<ProxyDef[]>([])
async function refreshProxyList() {
  proxyList.value = await proxyService.loadProxyList() || []
}
onMounted(refreshProxyList)

const initialForm: ProxyDef = {
  host: '',
  agent_name: '',
  agent_id: '',
  target: 'http://127.0.0.1:8080',
  replace_host: ''
}
const newProxy = ref<ProxyDef>({ ...initialForm })

const protocol = location.protocol

effect(() => { newProxy.value.agent_name = agentStore.agentInstance?.name || '' })

async function handleSubmit() {
  try {
    await proxyService.createProxy(newProxy.value)
    await refreshProxyList()
    // Reset form
    newProxy.value = { ...initialForm }
  } catch (error) {
    console.error('Failed to create proxy:', error)
    alert('Failed to create proxy: ' + error)
  }
}

async function deleteProxy(host: string) {
  try {
    await proxyService.deleteProxy(host)
    await refreshProxyList()
  } catch (error) {
    console.error('Failed to delete proxy:', error)
  }
}

function copyProxyInfo(proxy: ProxyDef) {
  Object.assign(newProxy.value, proxy)
}
</script>

<template>
  <div class="proxy-manager">
    <div class="proxy-form">
      <h2>Create New Proxy</h2>
      <form @submit.prevent="handleSubmit">
        <div class="form-group">
          <label>Hostname:</label>
          <input v-model="newProxy.host" required placeholder="e.g., example.com">
        </div>
        <div class="form-group">
          <label>Agent Name:</label>
          <input v-model="newProxy.agent_name" required>
          <button type="button" @click="newProxy.agent_name = String(agentStore.agentInstance?.name)">Current</button>
        </div>
        <div class="form-group">
          <label>Agent ID:</label>
          <input v-model="newProxy.agent_id">
          <button type="button" @click="newProxy.agent_id = String(agentStore.agentInstance?.id || '')">Current</button>
        </div>
        <div class="form-group">
          <label>Target:</label>
          <input v-model="newProxy.target" required placeholder="e.g., http://target-server.com">
        </div>
        <div class="form-group">
          <label>Replace Host:</label>
          <input v-model="newProxy.replace_host" required>
          <button type="button" @click="newProxy.replace_host = newProxy.host">HostName</button>
          <button type="button"
            @click="newProxy.replace_host = newProxy.target.replace(/^\w+:?\/+/, '').replace(/\/.*$/, '')">Target</button>
        </div>
        <button type="submit">Create Proxy</button>
      </form>
    </div>

    <div class="proxy-list">
      <h2>
        Proxy List
        <button @click="refreshProxyList" class="refresh">Refresh</button>
      </h2>
      <table>
        <thead>
          <tr>
            <th>Host</th>
            <th>Agent</th>
            <th>Target</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="proxy in proxyList" :key="proxy.replace_host">
            <td>
              <a :href="protocol + '//' + proxy.host" target="_blank">
                {{ proxy.host }}
              </a>
            </td>
            <td>{{ proxy.agent_name }} ({{ proxy.agent_id }})</td>
            <td>{{ proxy.target }}</td>
            <td>
              <button @click="deleteProxy(proxy.replace_host)" class="delete">Delete</button>
              <button @click="copyProxyInfo(proxy)" class="copy">Copy Info</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.proxy-manager {
  display: grid;
  grid-template-columns: 500px 1fr;
  gap: 2rem;
  padding: 1rem;
}

.proxy-list {
  overflow: auto;
  contain: size;
}

h2 {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: normal;
}

.form-group {
  margin-bottom: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.form-group label {
  width: 100px;
  margin-bottom: 0.5rem;
  text-wrap: nowrap;
}

.form-group input {
  flex: 1 0 100px;
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
}

button.delete {
  margin-right: 0.5rem;
}

button.delete:hover {
  color: white;
  background-color: #da190b;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th,
td {
  padding: 0.75rem;
  text-align: left;
  border-bottom: 1px solid #ddd;
}

th {
  background-color: #f5f5f5;
}
</style>
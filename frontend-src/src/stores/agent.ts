import { defineStore } from 'pinia'
import { ref, computed, shallowRef } from 'vue'
import { PtyService } from '../services/pty.service'

export interface AgentDef {
  id: number
  name: string
  user_agent: string
  is_upgradable: boolean
  join_at: string
  remote_addr: string
}

export const useAgentStore = defineStore('agent', () => {
  const apiKey = ref(localStorage.getItem('api_key') || '')
  const agentInstances = ref<AgentDef[]>([])
  const agentId = ref(-1)

  const agentInstance = computed(() => agentInstances.value.find(x => x.id === agentId.value))
  const agentName = computed(() => agentInstance.value?.name)

  const ptyService = shallowRef<PtyService | null>(null)

  async function reloadAgentInstances() {
    localStorage.setItem('api_key', apiKey.value)
    try {
      const res = await fetch(`./api/agent/`, {
        headers: { 'X-API-Key': apiKey.value }
      })
      agentInstances.value = await res.json() as AgentDef[]
    } catch (err) {
      console.error(err)
      agentInstances.value = []
    }
  }

  function connectPtyService() {
    if (ptyService.value) {
      ptyService.value.close()
      ptyService.value = null
      return
    }

    if (agentInstance.value) {
      const newPtyService = new PtyService(apiKey.value, agentInstance.value.name, agentInstance.value.id)
      newPtyService.connect()
      ptyService.value = newPtyService
    }
  }

  return {
    apiKey,
    agentInstances,
    agentId,
    agentInstance,
    agentName,
    reloadAgentInstances,

    ptyService,
    connectPtyService,
  }
})
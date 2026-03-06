import { defineStore } from 'pinia'
import { computed, ref, shallowRef } from 'vue'
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
  const selectedAgentId = ref(-1)
  const selectedAgent = computed(() => agentInstances.value.find(a => a.id === selectedAgentId.value))

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

  function connectPtyService(agent: AgentDef) {
    if (ptyService.value) {
      ptyService.value.close()
      ptyService.value = null
      return
    }

    const newPtyService = new PtyService(apiKey.value, agent.name, agent.id)
    newPtyService.connect()
    ptyService.value = newPtyService
  }

  return {
    apiKey,
    agentInstances,
    selectedAgentId,
    selectedAgent,
    reloadAgentInstances,
    ptyService,
    connectPtyService,
  }
})

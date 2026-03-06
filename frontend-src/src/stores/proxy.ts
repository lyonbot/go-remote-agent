import { defineStore } from 'pinia'
import { effect, ref } from 'vue'
import { useAgentStore } from './agent'
import { type ProxyDef, ProxyService } from '../services/proxy.service'

export const useProxyStore = defineStore('proxy', () => {
  const agentStore = useAgentStore()
  const proxyService = new ProxyService(agentStore.apiKey)
  effect(() => { proxyService.apiKey = agentStore.apiKey })

  const proxyList = ref<ProxyDef[]>([])

  async function refreshProxyList() {
    proxyList.value = await proxyService.loadProxyList() || []
  }

  async function createProxy(proxy: ProxyDef) {
    await proxyService.createProxy(proxy)
    await refreshProxyList()
  }

  async function deleteProxy(host: string) {
    await proxyService.deleteProxy(host)
    await refreshProxyList()
  }

  async function saveProxyList() {
    const res = await fetch(`./api/saveConfig`, {
      method: 'POST',
      headers: { 'X-API-Key': agentStore.apiKey }
    })
    if (!res.ok) {
      alert('Failed to save proxy list: ' + (await res.text()))
      return
    }
    alert('Saved')
  }

  // Dialog state
  const dialogOpen = ref(false)
  const dialogPrefill = ref<Partial<ProxyDef>>({})

  function openNewProxy(prefill?: Partial<ProxyDef>) {
    dialogPrefill.value = prefill || {
      agent_name: agentStore.selectedAgent?.name || '',
    }
    dialogOpen.value = true
  }

  function closeNewProxy() {
    dialogOpen.value = false
    dialogPrefill.value = {}
  }

  return {
    proxyList,
    refreshProxyList,
    createProxy,
    deleteProxy,
    saveProxyList,
    dialogOpen,
    dialogPrefill,
    openNewProxy,
    closeNewProxy,
  }
})

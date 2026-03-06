import { ref, shallowRef } from "vue"
import { defineStore } from "pinia"

export const useConfigStore = defineStore('config', () => {
  const apiKey = ref(localStorage.getItem('api_key') || '')
  const config = shallowRef({
    proxyServerHost: '',
  })

  async function loadConfig() {
    const res = await fetch(`./api/config`, {
      headers: { 'X-API-Key': apiKey.value }
    })
    config.value = await res.json()
  }

  return {
    apiKey,
    config,
    loadConfig,
  }
})
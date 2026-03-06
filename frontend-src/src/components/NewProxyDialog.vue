<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { useProxyStore } from '@/stores/proxy'
import { useAgentStore } from '@/stores/agent'
import type { ProxyDef } from '@/services/proxy.service'
import { useConfigStore } from '@/stores/config'

const proxyStore = useProxyStore()
const agentStore = useAgentStore()
const configStore = useConfigStore()

const proxyServerHost = computed(() => configStore.config.proxyServerHost || '')
if (!proxyServerHost.value) configStore.loadConfig()

const suggestedHost = computed(() => {
  if (!proxyServerHost.value) return ''
  if (!proxyServerHost.value.includes('*')) return ''

  let port = /:(\d+)/.exec(form.value.target)?.[1]
  if (!port) return ''

  let name = (form.value.agent_name)?.replace(/[^a-zA-Z0-9]+/g, '')
  if (!name) return ''

  const host = proxyServerHost.value.replace('*', `${name}-${port}`)
  return host
})

const initialForm = (): ProxyDef => ({
  host: '',
  agent_name: proxyStore.dialogPrefill.agent_name || agentStore.selectedAgent?.name || '',
  agent_id: '', // proxyStore.dialogPrefill.agent_id || String(agentStore.selectedAgent?.id || ''),
  target: proxyStore.dialogPrefill.target || 'http://127.0.0.1:8080',
  replace_host: proxyStore.dialogPrefill.replace_host || '',
  ...proxyStore.dialogPrefill,
})

const form = ref<ProxyDef>(initialForm())

watch(() => proxyStore.dialogOpen, (open) => {
  if (open) {
    form.value = initialForm()
  }
})

function contextAgent() {
  return agentStore.selectedAgent
}

async function handleSubmit() {
  try {
    await proxyStore.createProxy(form.value)
    proxyStore.closeNewProxy()
  } catch (error) {
    console.error('Failed to create proxy:', error)
    alert('Failed to create proxy: ' + error)
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="proxyStore.dialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      @click.self="proxyStore.closeNewProxy()">
      <div class="panel p-6 w-[480px] flex flex-col gap-4" @click.stop>
        <div class="flex items-center justify-between">
          <h2 class="section-title">New Proxy</h2>
          <button @click="proxyStore.closeNewProxy()" class="btn-sm btn-ghost">✕</button>
        </div>
        <form @submit.prevent="handleSubmit" class="flex flex-col gap-3">
          <div class="flex items-center gap-2">
            <label class="w-28 shrink-0 text-xs text-fg-muted">Hostname</label>
            <input v-model="form.host" required placeholder="example.com" class="field flex-1">
          </div>
          <div class="flex items-center gap-2 -mt-2" v-if="suggestedHost" @click="form.host = suggestedHost">
            <label class="w-28 shrink-0 text-xs text-fg-muted"></label>
            <span class="text-xs text-fg-dim cursor-pointer hover:underline flex-1">Suggestion: {{ suggestedHost }}</span>
          </div>
          <div class="flex items-center gap-2">
            <label class="w-28 shrink-0 text-xs text-fg-muted">Agent Name</label>
            <input v-model="form.agent_name" required class="field flex-1">
            <button type="button" @click="form.agent_name = contextAgent()?.name || ''"
              class="btn-sm btn-ghost shrink-0">Current</button>
          </div>
          <div class="flex items-center gap-2">
            <label class="w-28 shrink-0 text-xs text-fg-muted">Agent ID</label>
            <input v-model="form.agent_id" class="field flex-1 font-mono">
            <button type="button" @click="form.agent_id = String(contextAgent()?.id || '')"
              class="btn-sm btn-ghost shrink-0">Current</button>
          </div>
          <div class="flex items-center gap-2">
            <label class="w-28 shrink-0 text-xs text-fg-muted">Target</label>
            <input v-model="form.target" required placeholder="http://127.0.0.1:8080" class="field flex-1 font-mono">
          </div>
          <div class="flex items-center gap-2">
            <label class="w-28 shrink-0 text-xs text-fg-muted">Replace Host</label>
            <input v-model="form.replace_host" required class="field flex-1 font-mono">
            <button type="button" @click="form.replace_host = form.host" class="btn-sm btn-ghost shrink-0">Host</button>
            <button type="button" @click="form.replace_host = form.target.replace(/^\w+:?\/+/, '').replace(/\/.*$/, '')"
              class="btn-sm btn-ghost shrink-0">Target</button>
          </div>
          <div class="flex gap-2 mt-1">
            <button type="submit" class="btn btn-primary flex-1">Create Proxy</button>
            <button type="button" @click="proxyStore.closeNewProxy()" class="btn btn-ghost">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>

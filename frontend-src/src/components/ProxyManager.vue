<script lang="ts" setup>
import { onMounted } from 'vue'
import { useProxyStore } from '@/stores/proxy'

const proxyStore = useProxyStore()

const protocol = location.protocol

onMounted(proxyStore.refreshProxyList)

const confirm = (msg: string) => window.confirm(msg)
</script>

<template>
  <div class="panel p-4 flex flex-col overflow-hidden h-full">
    <div class="flex items-center gap-2 mb-4 shrink-0">
      <h2 class="section-title flex-1">Proxies</h2>
      <button @click="proxyStore.openNewProxy()" class="btn-sm btn-primary">+ New</button>
      <button @click="proxyStore.refreshProxyList()" class="btn-sm btn-ghost">Refresh</button>
      <button @click="proxyStore.saveProxyList()" class="btn-sm btn-ghost">Save</button>
    </div>
    <div class="overflow-auto flex-1">
      <table class="w-full text-sm border-collapse">
        <thead class="sticky top-0">
          <tr class="bg-raised">
            <th class="px-3 py-2 text-left text-xs font-semibold text-fg-muted uppercase tracking-wider">Host</th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-fg-muted uppercase tracking-wider">Agent</th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-fg-muted uppercase tracking-wider">Target</th>
            <th class="px-3 py-2 text-left text-xs font-semibold text-fg-muted uppercase tracking-wider"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="proxy in proxyStore.proxyList" :key="proxy.host" class="border-t border-border/50 hover:bg-raised/40 transition-colors">
            <td class="px-3 py-2 font-mono text-xs">
              <a :href="protocol + '//' + proxy.host" target="_blank" class="text-primary-light hover:text-primary hover:underline">
                {{ proxy.host }}
              </a>
            </td>
            <td class="px-3 py-2 text-fg-dim text-xs font-mono">
              {{ proxy.agent_name }}
              <span class="text-fg-subtle" v-if="proxy.agent_id">({{ proxy.agent_id }})</span>
            </td>
            <td class="px-3 py-2 text-fg-muted text-xs font-mono">{{ proxy.target }}</td>
            <td class="px-3 py-2">
              <div class="flex gap-1.5">
                <button @click="proxyStore.openNewProxy({ ...proxy })" class="btn-sm btn-ghost">Copy</button>
                <button @click="confirm(`Delete proxy ${proxy.host}?`) && proxyStore.deleteProxy(proxy.host)" class="btn-sm btn-danger">Del</button>
                <button @click="proxyStore.recreateProxy(proxy)" class="btn-sm btn-warning">Recreate</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

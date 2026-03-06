<script setup lang="ts">
import { watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAgentStore } from '@/stores/agent'
import ProxyManager from '@/components/ProxyManager.vue'
import NewProxyDialog from '@/components/NewProxyDialog.vue'

const router = useRouter()
const agentStore = useAgentStore()

watch(
  () => agentStore.ptyService,
  (service) => {
    if (service) {
      router.push(`/terminal/${service.agentId}`)
    } else {
      router.push('/')
    }
  },
)
</script>

<template>
  <div class="flex flex-col h-screen bg-base text-fg p-4 gap-4 overflow-hidden">
    <div class="flex gap-4 flex-1 min-h-0">
      <RouterView class="flex-1 min-h-0 min-w-0" />
      <ProxyManager class="w-[440px] shrink-0" />
    </div>
    <NewProxyDialog />
  </div>
</template>

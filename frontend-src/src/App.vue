<script setup lang="ts">
import { ref, watch } from 'vue'
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

const mobileShowProxyManager = ref(false)
</script>

<template>
  <div class="flex flex-col h-screen bg-base text-fg p-4 gap-4 overflow-hidden">
    <div class="flex gap-4 flex-1 min-h-0">
      <RouterView class="flex-1 min-h-0 min-w-0" />
      <ProxyManager class="w-[440px] shrink-0 hidden lg:block" />

      <!-- Mobile overlay -->
      <Transition name="fade">
        <div v-if="mobileShowProxyManager" class="lg:hidden fixed inset-0 bg-black/60 z-40"
          @click="mobileShowProxyManager = false" />
      </Transition>
      <Transition name="slide-up">
        <ProxyManager v-if="mobileShowProxyManager"
          class="lg:hidden fixed bottom-0 left-0 right-0 z-50 max-h-[80vh] overflow-y-auto rounded-t-xl" />
      </Transition>

      <!-- Mobile trigger button -->
      <button class="lg:hidden btn btn-ghost fixed bottom-4 right-4 z-30 shadow-lg"
        @click="mobileShowProxyManager = !mobileShowProxyManager">
        Proxy
      </button>
    </div>
    <NewProxyDialog />
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.slide-up-enter-active, .slide-up-leave-active { transition: transform 0.25s ease; }
.slide-up-enter-from, .slide-up-leave-to { transform: translateY(100%); }
</style>

<script lang="ts" setup>
import { useAgentStore } from '@/stores/agent';
import { computed, ref } from 'vue';

const agentStore = useAgentStore()
const fs = computed(() => agentStore.ptyService!.fs)

const path = ref('')
const progress = ref(0)
const isBusy = ref(false)
const error = ref('')

async function handleDownload() {
  isBusy.value = true
  progress.value = 0
  error.value = ''
  try {
    fs.value.downloadFile(path.value, {
      onprogress: val => progress.value = val
    })
  } catch (e) {
    error.value = String(e)
    console.error(e)
  }
  isBusy.value = false
}

const uploadFileSelector = ref<HTMLInputElement>()
function handleOpenUploadFileSelector() {
  uploadFileSelector.value!.value = ''
  uploadFileSelector.value!.click()
}
function handleUploadFile() {
  const file = uploadFileSelector.value?.files?.[0]
  if (!file) return

  isBusy.value = true
  progress.value = 0
  error.value = ''

  const reader = new FileReader()
  reader.onload = async (e) => {
    const data = new Uint8Array(e.target!.result as ArrayBuffer)
    try {
      console.log('uploading file', file.name, data.length, 'bytes')
      await fs.value.uploadFile(path.value, data, {
        onprogress: val => progress.value = val
      })
    } catch (e) {
      error.value = String(e)
      console.error(e)
    }
    isBusy.value = false
  }
  reader.onerror = (e) => {
    console.error(e)
    error.value = String(e)
    isBusy.value = false
  }
  reader.readAsArrayBuffer(file)
}
</script>

<template>
  <div class="flex items-stretch gap-2 shrink-0">
    <input type="text" v-model="path" placeholder="/path/to/file" class="field flex-1 font-mono" />
    <button @click="handleDownload" :disabled="isBusy" class="btn btn-ghost">Download</button>
    <button @click="handleOpenUploadFileSelector" :disabled="isBusy" class="btn btn-ghost">Upload</button>

    <div class="relative min-w-24 rounded border border-border overflow-hidden bg-raised">
      <div
        class="absolute inset-y-0 left-0 transition-[width] duration-300"
        :class="{ 'bg-primary': !isBusy && !error, 'bg-info': isBusy, 'bg-danger': !!error }"
        :style="{ width: progress + '%' }"
      ></div>
      <span v-if="error" class="relative z-10 px-2 text-xs text-white leading-none flex items-center h-full font-mono">
        {{ error }}
      </span>
    </div>

    <input type="file" ref="uploadFileSelector" @change="handleUploadFile" class="absolute w-px h-px opacity-0 pointer-events-none" />
  </div>
</template>

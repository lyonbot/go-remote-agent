<script lang="ts" setup>
import { useAgentStore } from '@/stores/agent';
import { computed, ref } from 'vue';

const agentStore = useAgentStore()
const fs = computed(() => agentStore.ptyService!.fs) // assume ptyService is ready

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
  <div class="file-transfer-toolbar">
    <input type="text" v-model="path" placeholder="Path" class="path-input" />
    <button @click="handleDownload" :disabled="isBusy">Download</button>
    <button @click="handleOpenUploadFileSelector" :disabled="isBusy">Upload</button>

    <div class="progress-bar">
      <div class="progress" :style="{ width: progress + '%' }" :class="{ isBusy, hasError: !!error }"></div>
      <div class="progress-error" v-if="error">{{ error }}</div>
    </div>

    <input type="file" ref="uploadFileSelector" @change="handleUploadFile"
      style="width: 1px; height: 1px; opacity: 0; position: absolute" />
  </div>
</template>

<style scoped>
.file-transfer-toolbar {
  display: flex;
  gap: 0.5rem;
}

.path-input {
  flex: 1;
}

.progress-bar {
  min-width: 100px;
  padding: 0 16px;
  align-self: stretch;
  border: 1px solid #ccc;
  position: relative;
}

.progress {
  left: 0;
  top: 0;
  height: 100%;
  background-color: #4CAF50;
  transition: width 0.3s;
  position: absolute;
}

.progress.isBusy {
  background-color: #008CBA;
}

.progress.hasError {
  background-color: #f44336;
}

.progress-error {
  z-index: 1;
  color: white;
  text-shadow: 0 1px 5px #000;
}
</style>
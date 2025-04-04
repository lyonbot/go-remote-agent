<script lang="ts" setup>
import { type PtyTermOptions } from '@/services/pty.service';
import { ref } from 'vue';
import Terminal from './Terminal.vue'

const form = ref({
  cmd: 'sh',
  args: '',
  env: 'TERM=xterm-256color',
  inherit_env: true
})

const termOptions = ref<PtyTermOptions>()

const created = ref(false)
function createTerminal() {
  if (created.value) return

  termOptions.value = {
    cmd: form.value.cmd,
    args: form.value.args ? form.value.args.split('\n') : [],
    env: form.value.env ? form.value.env.split('\n') : [],
    inherit_env: form.value.inherit_env
  }
  created.value = true
}
</script>

<template>
  <div class="terminal-container" :class="{ created }">
    <form v-if="!created" @submit.prevent="createTerminal()" class="form">
      <div class="form-group">
        <label for="cmd">Command:</label>
        <input type="text" id="cmd" v-model="form.cmd" required>
      </div>

      <div class="form-group">
        <label for="args">Arguments (one per line):</label>
        <textarea id="args" v-model="form.args" rows="3"></textarea>
      </div>

      <div class="form-group">
        <label for="env">Environment Variables (one per line):</label>
        <textarea id="env" v-model="form.env" rows="3"></textarea>
      </div>

      <div class="form-group">
        <label>
          <input type="checkbox" v-model="form.inherit_env">
          Inherit Environment Variables
        </label>
      </div>

      <button type="submit">Create Terminal</button>
    </form>

    <Terminal v-if="created && termOptions" :options="termOptions" />
  </div>
</template>

<style scoped>
.terminal-container {
  position: relative;
  display: flex;
}

.terminal-container.created {
  padding: 16px;
  border-radius: 4px;
  background-color: #000;
}

.form {
  margin: auto;
  padding: 1rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
}
</style>
<script lang="ts" setup>
import { ref } from 'vue'
import { parse as shellParse } from 'shell-quote'
import { type PtyTermOptions } from '@/services/pty.service'
import Terminal from './Terminal.vue'

function parseShellCommand(input: string): string[] {
  return shellParse(input).filter((t): t is string => typeof t === 'string')
}

// ─── History ─────────────────────────────────────────────────────────────
const HISTORY_KEY = 'terminal-cmd-history'
const MAX_HISTORY = 30

const history = ref<string[]>(JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]'))

function addToHistory(cmd: string) {
  const t = cmd.trim()
  if (!t) return
  history.value = [t, ...history.value.filter(h => h !== t)].slice(0, MAX_HISTORY)
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
}

function removeFromHistory(cmd: string) {
  history.value = history.value.filter(h => h !== cmd)
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
}

// ─── Mode & form state ────────────────────────────────────────────────────
const mode = ref<'quick' | 'advanced'>('quick')

const shellCmd = ref('sh')
const showHistory = ref(false)
let blurTimer: ReturnType<typeof setTimeout> | null = null

function onInputFocus() {
  if (blurTimer) clearTimeout(blurTimer)
  if (history.value.length) showHistory.value = true
}

function onInputBlur() {
  blurTimer = setTimeout(() => { showHistory.value = false }, 150)
}

function keepDropdownOpen() {
  if (blurTimer) clearTimeout(blurTimer)
}

function fillFromHistory(cmd: string) {
  shellCmd.value = cmd
  showHistory.value = false
}

const form = ref({
  cmd: 'sh',
  args: '',
  env: 'TERM=xterm-256color',
  inherit_env: true,
})

// ─── Launch ───────────────────────────────────────────────────────────────
const termOptions = ref<PtyTermOptions>()
const created = ref(false)

function createTerminal() {
  if (created.value) return

  let cmd: string
  let args: string[]

  if (mode.value === 'quick') {
    const tokens = parseShellCommand(shellCmd.value.trim())
    if (!tokens.length) return
    addToHistory(shellCmd.value.trim())
    cmd = tokens[0]
    args = tokens.slice(1)
  } else {
    cmd = form.value.cmd
    args = form.value.args ? form.value.args.split('\n').filter(Boolean) : []
  }

  termOptions.value = {
    cmd,
    args,
    env: form.value.env ? form.value.env.split('\n').filter(Boolean) : [],
    inherit_env: form.value.inherit_env,
  }
  created.value = true
}
</script>

<template>
  <div class="flex-1 min-h-0 flex rounded-lg overflow-hidden" :class="created ? 'bg-black' : 'panel'">
    <form v-if="!created" @submit.prevent="createTerminal()" class="m-auto flex flex-col gap-4 w-[380px] p-6">

      <!-- Header + mode tabs -->
      <div class="flex items-center gap-2">
        <h2 class="section-title flex-1">Terminal</h2>
        <div class="flex rounded overflow-hidden border border-border text-xs">
          <button
            type="button"
            @click="mode = 'quick'"
            :class="mode === 'quick' ? 'bg-control text-fg' : 'text-fg-muted hover:text-fg hover:bg-raised'"
            class="px-3 py-1 transition-colors"
          >Quick</button>
          <button
            type="button"
            @click="mode = 'advanced'"
            :class="mode === 'advanced' ? 'bg-control text-fg' : 'text-fg-muted hover:text-fg hover:bg-raised'"
            class="px-3 py-1 transition-colors"
          >Advanced</button>
        </div>
      </div>

      <!-- Quick mode: single shell command + history -->
      <div v-if="mode === 'quick'" class="flex flex-col gap-1">
        <label class="text-xs text-fg-muted">Shell Command</label>
        <div class="relative">
          <input
            v-model="shellCmd"
            required
            @focus="onInputFocus"
            @blur="onInputBlur"
            placeholder="sh -c 'echo hello'"
            class="field font-mono w-full pr-8"
            autocomplete="off"
          >
          <button
            v-if="history.length"
            type="button"
            @mousedown.prevent="keepDropdownOpen(); showHistory = !showHistory"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-fg-subtle hover:text-fg-dim transition-colors text-xs leading-none select-none"
            title="History"
          >{{ showHistory ? '▴' : '▾' }}</button>

          <!-- History dropdown -->
          <div
            v-if="showHistory && history.length"
            @mousedown.prevent="keepDropdownOpen()"
            class="absolute top-full left-0 right-0 z-20 mt-1 bg-surface border border-border rounded-md shadow-lg overflow-hidden"
          >
            <div class="max-h-52 overflow-y-auto">
              <div
                v-for="cmd in history"
                :key="cmd"
                @mousedown.prevent="fillFromHistory(cmd)"
                class="flex items-center gap-1 px-3 py-1.5 hover:bg-raised cursor-pointer group"
              >
                <span class="text-xs font-mono text-fg-dim flex-1 truncate">{{ cmd }}</span>
                <button
                  type="button"
                  @mousedown.stop.prevent="removeFromHistory(cmd)"
                  class="text-fg-subtle hover:text-danger opacity-0 group-hover:opacity-100 transition-opacity text-xs px-1 shrink-0"
                >✕</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Advanced mode -->
      <template v-else>
        <div class="flex flex-col gap-1">
          <label class="text-xs text-fg-muted">Command</label>
          <input v-model="form.cmd" required class="field font-mono">
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-xs text-fg-muted">Arguments <span class="text-fg-subtle">(one per line)</span></label>
          <textarea v-model="form.args" rows="3" class="field font-mono"></textarea>
        </div>
      </template>

      <!-- Shared: env + inherit -->
      <div class="flex flex-col gap-1">
        <label class="text-xs text-fg-muted">Environment <span class="text-fg-subtle">(one per line)</span></label>
        <textarea v-model="form.env" rows="2" class="field font-mono"></textarea>
      </div>
      <label class="flex items-center gap-2 text-sm text-fg-dim cursor-pointer">
        <input type="checkbox" v-model="form.inherit_env" class="w-3.5 h-3.5 accent-primary">
        Inherit Environment
      </label>

      <button type="submit" class="btn btn-primary mt-1">Launch Terminal</button>
    </form>

    <div v-if="created && termOptions" class="flex-1 min-h-0 p-4">
      <Terminal :options="termOptions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, defineComponent, h, onUnmounted } from 'vue'
import { useAgentStore } from '@/stores/agent'
import type { FileInfo } from '@/services/types'

const agentStore = useAgentStore()
const fs = computed(() => agentStore.ptyService?.fs ?? null)

// ─── Tree node ────────────────────────────────────────────────────────────────

interface TreeNode {
  path: string
  name: string
  isDir: boolean
  size: number
  mtime: number
  mode: number
  children: TreeNode[] | null  // null = not loaded
  expanded: boolean
  loading: boolean
}

function makeNode(info: FileInfo): TreeNode {
  const isDir = (info.mode >>> 31) === 1
  const name = info.path.split('/').pop() || info.path
  return { path: info.path, name, isDir, size: info.size, mtime: info.mtime, mode: info.mode, children: null, expanded: false, loading: false }
}

function sortNodes(nodes: TreeNode[]): TreeNode[] {
  return nodes.sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

// ─── State ────────────────────────────────────────────────────────────────────

const rootPath = ref('/')
const rootNode = shallowRef<TreeNode>({
  path: '/', name: '/', isDir: true, size: 0, mtime: 0, mode: 0x80000000,
  children: null, expanded: true, loading: false,
})
const renderKey = ref(0)
const error = ref('')

function triggerRerender() { renderKey.value++ }

// ─── Tree operations ──────────────────────────────────────────────────────────

async function loadChildren(node: TreeNode) {
  if (!fs.value) return
  node.loading = true
  triggerRerender()
  try {
    const items = await fs.value.listDir(node.path)
    node.children = sortNodes(items.map(makeNode))
  } catch (e: any) {
    error.value = String(e)
  } finally {
    node.loading = false
    triggerRerender()
  }
}

async function toggleExpand(node: TreeNode) {
  if (!node.isDir) return
  node.expanded = !node.expanded
  if (node.expanded && node.children === null) await loadChildren(node)
  else triggerRerender()
}

async function navigateToRoot() {
  rootNode.value.path = rootPath.value
  rootNode.value.children = null
  rootNode.value.expanded = true
  await loadChildren(rootNode.value)
  triggerRerender()
}

navigateToRoot()

// ─── Context menu ─────────────────────────────────────────────────────────────

interface CtxMenu { x: number; y: number; node: TreeNode | null; parentNode: TreeNode | null }
const ctxMenu = ref<CtxMenu | null>(null)

function openCtxMenu(e: MouseEvent, node: TreeNode | null, parent: TreeNode | null) {
  e.preventDefault()
  e.stopPropagation()
  ctxMenu.value = { x: e.clientX, y: e.clientY, node, parentNode: parent }
}
function closeCtxMenu() { ctxMenu.value = null }

function ctxDownload() {
  const node = ctxMenu.value?.node
  closeCtxMenu()
  if (!node || !fs.value) return
  fs.value.downloadFile(node.path, {})
}

function ctxCopyPath() {
  const path = ctxMenu.value?.node?.path
  closeCtxMenu()
  if (path) navigator.clipboard.writeText(path)
}

async function ctxDelete() {
  const node = ctxMenu.value?.node
  const parent = ctxMenu.value?.parentNode
  closeCtxMenu()
  if (!node || !fs.value) return
  if (!confirm(`Delete "${node.path}"?`)) return
  try {
    await fs.value.deleteFile(node.path)
    if (parent?.children) {
      parent.children = parent.children.filter(c => c.path !== node.path)
      triggerRerender()
    }
  } catch (e: any) { error.value = String(e) }
}

async function doNewFolder(dirNode: TreeNode) {
  const name = prompt('Folder name:')
  if (!name || !fs.value) return
  const newPath = dirNode.path.replace(/\/$/, '') + '/' + name
  try {
    await fs.value.mkdir(newPath)
    const newNode = makeNode({ path: newPath, size: 0, mtime: Date.now() / 1000, mode: 0x80000000 })
    if (dirNode.children !== null) {
      dirNode.children = sortNodes([...dirNode.children, newNode])
      triggerRerender()
    }
  } catch (e: any) { error.value = String(e) }
}

async function doNewTextFile(dirNode: TreeNode) {
  const name = prompt('File name:')
  if (!name || !fs.value) return
  const newPath = dirNode.path.replace(/\/$/, '') + '/' + name
  try {
    await fs.value.writeTextFile(newPath, '')
    const newNode = makeNode({ path: newPath, size: 0, mtime: Date.now() / 1000, mode: 0o644 })
    if (dirNode.children !== null) {
      dirNode.children = sortNodes([...dirNode.children, newNode])
      triggerRerender()
    }
    openEditor(newNode)
  } catch (e: any) { error.value = String(e) }
}

// ─── File type detection ──────────────────────────────────────────────────────

const SIZE_3MB = 3 * 1024 * 1024

const IMAGE_EXTS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'ico', 'bmp', 'tiff', 'avif'])
const AUDIO_EXTS = new Set(['mp3', 'wav', 'ogg', 'flac', 'aac', 'm4a', 'opus', 'weba'])
const VIDEO_EXTS = new Set(['mp4', 'webm', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ogv'])
const ARCHIVE_EXTS = new Set(['zip', 'tar', 'gz', 'bz2', 'xz', 'rar', '7z', 'tgz', 'tbz2', 'zst', 'br', 'lz4'])
const BINARY_EXTS = new Set(['exe', 'bin', 'so', 'dylib', 'dll', 'o', 'a', 'lib', 'class', 'pyc', 'wasm', 'pdf'])

type MediaType = 'image' | 'audio' | 'video' | 'archive' | 'binary'

function getMediaType(ext: string): MediaType | null {
  if (IMAGE_EXTS.has(ext)) return 'image'
  if (AUDIO_EXTS.has(ext)) return 'audio'
  if (VIDEO_EXTS.has(ext)) return 'video'
  if (ARCHIVE_EXTS.has(ext)) return 'archive'
  if (BINARY_EXTS.has(ext)) return 'binary'
  return null
}

function getMimeType(ext: string): string {
  const m: Record<string, string> = {
    png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif',
    webp: 'image/webp', svg: 'image/svg+xml', ico: 'image/x-icon', bmp: 'image/bmp',
    tiff: 'image/tiff', avif: 'image/avif',
    mp3: 'audio/mpeg', wav: 'audio/wav', ogg: 'audio/ogg', flac: 'audio/flac',
    aac: 'audio/aac', m4a: 'audio/mp4', opus: 'audio/opus', weba: 'audio/webm',
    mp4: 'video/mp4', webm: 'video/webm', mkv: 'video/x-matroska',
    avi: 'video/x-msvideo', mov: 'video/quicktime', ogv: 'video/ogg',
  }
  return m[ext] ?? 'application/octet-stream'
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
  return `${(bytes / 1073741824).toFixed(1)} GB`
}

// ─── Large-file confirmation ──────────────────────────────────────────────────

const largeFileConfirm = ref<TreeNode | null>(null)

// ─── Media viewer ─────────────────────────────────────────────────────────────

interface MediaViewer {
  node: TreeNode
  type: MediaType
  url: string   // object URL, empty for archive/binary
  mimeType: string
}

const mediaViewer = ref<MediaViewer | null>(null)
const mediaViewerLoading = ref(false)

async function readBinaryFromFs(path: string, size: number): Promise<Uint8Array> {
  if (!fs.value) throw new Error('No fs service')
  const chunks: Uint8Array[] = []
  for (let offset = 0; offset < size;) {
    const chunk = await fs.value.downloadFileChunk(path, offset)
    if (chunk.data.length === 0) break
    chunks.push(chunk.data)
    offset += chunk.data.length
  }
  const all = new Uint8Array(chunks.reduce((acc, c) => acc + c.length, 0))
  let pos = 0
  for (const c of chunks) { all.set(c, pos); pos += c.length }
  return all
}

async function openMediaViewer(node: TreeNode, type: MediaType, ext: string) {
  if (!fs.value) return
  if (mediaViewer.value?.url) URL.revokeObjectURL(mediaViewer.value.url)
  editorNode.value = null
  const mimeType = getMimeType(ext)
  mediaViewer.value = { node, type, url: '', mimeType }
  mediaViewerLoading.value = true
  try {
    if (type === 'image' || type === 'audio' || type === 'video') {
      const data = await readBinaryFromFs(node.path, node.size)
      const blob = new Blob([data], { type: mimeType })
      mediaViewer.value.url = URL.createObjectURL(blob)
    }
  } catch (e: any) {
    error.value = String(e)
  } finally {
    mediaViewerLoading.value = false
  }
}

function closeMediaViewer() {
  if (mediaViewer.value?.url) URL.revokeObjectURL(mediaViewer.value.url)
  mediaViewer.value = null
}

function downloadMediaViewer() {
  const mv = mediaViewer.value
  if (!mv || !fs.value) return
  if (mv.url) {
    const link = document.createElement('a')
    link.href = mv.url
    link.download = mv.node.name
    link.click()
  } else {
    fs.value.downloadFile(mv.node.path, {})
  }
}

onUnmounted(() => {
  if (mediaViewer.value?.url) URL.revokeObjectURL(mediaViewer.value.url)
})

// ─── Text editor ──────────────────────────────────────────────────────────────

const editorNode = ref<TreeNode | null>(null)
const editorContent = ref('')
const editorSaving = ref(false)
const editorLoading = ref(false)
const editorDirty = ref(false)

async function loadIntoEditor(node: TreeNode) {
  if (!fs.value) return
  mediaViewer.value = null
  editorNode.value = node
  editorLoading.value = true
  editorDirty.value = false
  try {
    editorContent.value = await fs.value.readTextFile(node.path)
  } catch (e: any) {
    editorContent.value = ''
    error.value = String(e)
  } finally {
    editorLoading.value = false
  }
}

async function openEditor(node: TreeNode, asText = false) {
  if (node.isDir || !fs.value) return

  const ext = node.name.split('.').pop()?.toLowerCase() ?? ''

  // Non-text binary format → media viewer (unless forced as text)
  if (!asText) {
    const type = getMediaType(ext)
    if (type !== null) {
      await openMediaViewer(node, type, ext)
      return
    }
  }

  // Size guard: >3 MB → show confirmation overlay
  if (node.size > SIZE_3MB) {
    largeFileConfirm.value = node
    return
  }

  await loadIntoEditor(node)
}

async function saveEditor() {
  if (!editorNode.value || !fs.value) return
  editorSaving.value = true
  try {
    await fs.value.writeTextFile(editorNode.value.path, editorContent.value)
    editorDirty.value = false
  } catch (e: any) { error.value = String(e) }
  finally { editorSaving.value = false }
}

function closeEditor() {
  if (editorDirty.value && !confirm('Discard unsaved changes?')) return
  editorNode.value = null
}

// ─── Drag & drop ──────────────────────────────────────────────────────────────

const dragOverNode = ref<TreeNode | null>(null)

async function onDrop(e: DragEvent, node: TreeNode) {
  e.preventDefault()
  dragOverNode.value = null
  if (!node.isDir || !fs.value || !e.dataTransfer?.files.length) return
  const targetDir = node.path.replace(/\/$/, '')
  for (const file of Array.from(e.dataTransfer.files)) {
    const destPath = targetDir + '/' + file.name
    const buf = await file.arrayBuffer()
    try {
      await fs.value.uploadFile(destPath, new Uint8Array(buf), {})
      const newNode = makeNode({ path: destPath, size: file.size, mtime: file.lastModified / 1000, mode: 0o644 })
      if (node.children !== null) {
        node.children = sortNodes([...node.children.filter(c => c.path !== destPath), newNode])
        triggerRerender()
      }
    } catch (err: any) { error.value = String(err) }
  }
}

// ─── Recursive tree node component ────────────────────────────────────────────

function getFileIcon(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  const icons: Record<string, string> = {
    js: 'JS', ts: 'TS', vue: 'VUE', go: 'GO', py: 'PY', sh: 'SH', md: 'MD',
    json: '{}', yaml: 'YML', yml: 'YML', txt: 'TXT', log: 'LOG', css: 'CSS',
    html: 'HTM', rs: 'RS', c: 'C', cpp: 'C++', h: 'H',
  }
  return icons[ext] ?? '•'
}

const FileTreeNode: any = defineComponent({
  name: 'FileTreeNode',
  props: {
    node: Object,
    parent: Object,
    depth: { type: Number, default: 0 },
    dragOverNode: Object,
  },
  emits: ['toggle', 'open', 'ctx', 'dragover', 'dragleave', 'drop'],
  setup(props: any, { emit }: any) {
    return () => {
      const node = props.node
      const isDragTarget = props.dragOverNode === node
      const indent = props.depth * 14 + 6

      const rowEl = h('div', {
        class: [
          'flex items-center gap-1 py-0.5 pr-2 cursor-pointer select-none text-xs',
          'hover:bg-raised rounded',
          isDragTarget ? 'bg-primary/20' : '',
        ].join(' '),
        style: { paddingLeft: indent + 'px' },
        onClick: () => node.isDir ? emit('toggle', node) : emit('open', node),
        onDblclick: () => { if (!node.isDir) emit('open', node) },
        onContextmenu: (e: MouseEvent) => { e.preventDefault(); e.stopPropagation(); emit('ctx', e, node, props.parent) },
        onDragover: (e: DragEvent) => { if (node.isDir) { e.preventDefault(); emit('dragover', e, node) } },
        onDragleave: () => emit('dragleave'),
        onDrop: (e: DragEvent) => { e.preventDefault(); if (node.isDir) emit('drop', e, node) },
      }, [
        h('span', { class: 'text-fg-muted text-center shrink-0 w-4 text-[10px]' },
          node.loading ? '…' : node.isDir ? (node.expanded ? '▾' : '▸') : ''),
        h('span', {
          class: 'truncate ' + (node.isDir ? 'text-fg-dim' : 'text-fg'),
          style: 'flex:1;min-width:0'
        }, node.name),
        !node.isDir && node.size > 0 && h('span', { class: 'text-fg-subtle shrink-0 font-mono text-[10px]' }, formatSizeSmall(node.size)),
      ])

      const childrenEls = node.expanded && node.isDir && node.children?.map((child: TreeNode) =>
        h(FileTreeNode, {
          key: child.path,
          node: child,
          parent: node,
          depth: props.depth + 1,
          dragOverNode: props.dragOverNode,
          onToggle: (n: any) => emit('toggle', n),
          onOpen: (n: any) => emit('open', n),
          onCtx: (e: MouseEvent, n: any, p: any) => emit('ctx', e, n, p),
          onDragover: (e: DragEvent, n: any) => emit('dragover', e, n),
          onDragleave: () => emit('dragleave'),
          onDrop: (e: DragEvent, n: any) => emit('drop', e, n),
        })
      )

      return h('div', [rowEl, ...(childrenEls || [])])
    }
  }
})

function formatSizeSmall(bytes: number): string {
  if (!bytes) return ''
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(0)}K`
  return `${(bytes / 1048576).toFixed(1)}M`
}
</script>

<template>
  <div class="flex flex-col h-full min-h-0 bg-surface border border-border rounded-lg overflow-hidden"
    @click="closeCtxMenu">

    <!-- Header -->
    <div class="flex items-center gap-1.5 px-2 py-1.5 border-b border-border shrink-0">
      <span class="section-title">Files</span>
      <input v-model="rootPath" @keydown.enter="navigateToRoot"
        class="field flex-1 font-mono text-xs py-0.5 px-1.5 min-w-0" placeholder="/" />
      <button @click="navigateToRoot" class="btn-sm btn-ghost shrink-0" title="Go">→</button>
      <button @click="loadChildren(rootNode)" class="btn-sm btn-ghost shrink-0" title="Refresh">↺</button>
    </div>

    <!-- Error banner -->
    <div v-if="error"
      class="flex items-center gap-1 px-2 py-1 text-xs text-danger bg-danger/10 border-b border-danger/20 shrink-0">
      <span class="flex-1 truncate">{{ error }}</span>
      <button @click="error = ''" class="hover:text-fg leading-none">✕</button>
    </div>

    <div class="flex flex-1 min-h-0 overflow-hidden">

      <!-- Tree panel -->
      <div class="flex flex-col overflow-y-auto"
        :class="(editorNode || mediaViewer) ? 'w-44 shrink-0 border-r border-border' : 'flex-1 w-full'"
        @contextmenu.prevent="openCtxMenu($event, null, rootNode)">
        <div :key="renderKey" class="py-1">
          <FileTreeNode
            v-for="child in rootNode.children ?? []"
            :key="child.path"
            :node="child"
            :parent="rootNode"
            :depth="0"
            :drag-over-node="dragOverNode"
            @toggle="toggleExpand"
            @open="openEditor"
            @ctx="openCtxMenu"
            @dragover="(e: DragEvent, n: TreeNode) => { dragOverNode = n; e.preventDefault() }"
            @dragleave="dragOverNode = null"
            @drop="onDrop"
          />
          <div v-if="rootNode.loading" class="px-3 py-1.5 text-xs text-fg-subtle">Loading…</div>
          <div v-else-if="rootNode.children?.length === 0"
            class="px-3 py-1.5 text-xs text-fg-subtle italic">Empty directory</div>
        </div>
      </div>

      <!-- Text editor -->
      <div v-if="editorNode" class="flex flex-col flex-1 min-w-0 min-h-0">
        <div class="flex items-center gap-1.5 px-2 py-1 border-b border-border shrink-0">
          <span class="font-mono text-xs text-fg-dim truncate flex-1" :title="editorNode.path">
            {{ editorNode.name }}
          </span>
          <span v-if="editorDirty" class="text-xs text-warning" title="Unsaved changes">●</span>
          <button @click="saveEditor" :disabled="editorSaving || !editorDirty" class="btn-sm btn-primary">
            {{ editorSaving ? 'Saving…' : 'Save' }}
          </button>
          <button @click="closeEditor" class="btn-sm btn-ghost">✕</button>
        </div>
        <div v-if="editorLoading" class="flex-1 flex items-center justify-center text-xs text-fg-subtle">
          Loading…
        </div>
        <textarea v-else v-model="editorContent"
          @input="editorDirty = true"
          @keydown.ctrl.s.prevent="saveEditor"
          @keydown.meta.s.prevent="saveEditor"
          class="flex-1 min-h-0 w-full bg-base text-fg font-mono text-xs p-3 resize-none outline-none"
          spellcheck="false" />
      </div>

      <!-- Media viewer -->
      <div v-else-if="mediaViewer" class="flex flex-col flex-1 min-w-0 min-h-0">
        <div class="flex items-center gap-1.5 px-2 py-1 border-b border-border shrink-0">
          <span class="font-mono text-xs text-fg-dim truncate flex-1" :title="mediaViewer.node.path">
            {{ mediaViewer.node.name }}
          </span>
          <button @click="downloadMediaViewer" class="btn-sm btn-ghost" title="Download">↓ Download</button>
          <button @click="openEditor(mediaViewer.node, true)" class="btn-sm btn-ghost" title="Open as text">TXT</button>
          <button @click="closeMediaViewer" class="btn-sm btn-ghost">✕</button>
        </div>
        <div v-if="mediaViewerLoading" class="flex-1 flex items-center justify-center text-xs text-fg-subtle">
          Loading…
        </div>
        <div v-else class="flex-1 min-h-0 overflow-auto flex flex-col items-center justify-start p-4 gap-3">
          <!-- Image preview -->
          <img v-if="mediaViewer.type === 'image' && mediaViewer.url"
            :src="mediaViewer.url" class="max-w-full object-contain rounded" />
          <!-- Audio player -->
          <audio v-else-if="mediaViewer.type === 'audio' && mediaViewer.url"
            :src="mediaViewer.url" controls class="w-full mt-8" />
          <!-- Video player -->
          <video v-else-if="mediaViewer.type === 'video' && mediaViewer.url"
            :src="mediaViewer.url" controls class="max-w-full rounded" />
          <!-- Archive / binary: info + download -->
          <div v-else class="flex flex-col items-center gap-3 text-center mt-8">
            <span class="font-mono text-2xl text-fg-subtle">
              {{ mediaViewer.type === 'archive' ? 'PKG' : 'BIN' }}
            </span>
            <p class="text-xs text-fg-muted capitalize">{{ mediaViewer.type }} file</p>
            <p class="font-mono text-xs text-fg-subtle">{{ formatSize(mediaViewer.node.size) }}</p>
            <button @click="downloadMediaViewer" class="btn btn-primary">Download</button>
          </div>
          <!-- Size footnote for preview types -->
          <p v-if="mediaViewer.type !== 'archive' && mediaViewer.type !== 'binary'"
            class="text-xs text-fg-subtle font-mono">{{ formatSize(mediaViewer.node.size) }}</p>
        </div>
      </div>
    </div>
  </div>

  <!-- Context menu -->
  <Teleport to="body">
    <div v-if="ctxMenu"
      class="fixed z-50 bg-surface border border-border rounded-lg shadow-xl py-1 min-w-40 text-sm"
      :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
      @click.stop>
      <!-- File node -->
      <template v-if="ctxMenu.node && !ctxMenu.node.isDir">
        <button class="ctx-item" @click="ctxDownload">Download</button>
        <button class="ctx-item" @click="ctxCopyPath">Copy Path</button>
        <div class="border-t border-border my-1" />
        <button class="ctx-item text-danger hover:bg-danger/10" @click="ctxDelete">Delete</button>
      </template>
      <!-- Directory node -->
      <template v-else-if="ctxMenu.node?.isDir">
        <button class="ctx-item" @click="ctxCopyPath">Copy Path</button>
        <button class="ctx-item" @click="doNewFolder(ctxMenu.node!); closeCtxMenu()">New Folder</button>
        <button class="ctx-item" @click="doNewTextFile(ctxMenu.node!); closeCtxMenu()">New Text File</button>
        <div class="border-t border-border my-1" />
        <button class="ctx-item text-danger hover:bg-danger/10" @click="ctxDelete">Delete</button>
      </template>
      <!-- Background -->
      <template v-else>
        <button class="ctx-item" @click="ctxMenu?.parentNode && doNewFolder(ctxMenu.parentNode); closeCtxMenu()">
          New Folder
        </button>
        <button class="ctx-item" @click="ctxMenu?.parentNode && doNewTextFile(ctxMenu.parentNode); closeCtxMenu()">
          New Text File
        </button>
        <div class="border-t border-border my-1" />
        <button class="ctx-item" @click="loadChildren(rootNode); closeCtxMenu()">Refresh</button>
      </template>
    </div>
  </Teleport>

  <!-- Large-file confirmation overlay -->
  <Teleport to="body">
    <div v-if="largeFileConfirm"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      @click.self="largeFileConfirm = null">
      <div class="bg-surface border border-border rounded-lg p-5 shadow-xl w-80">
        <p class="text-sm text-fg font-medium mb-1">Large file</p>
        <p class="font-mono text-xs text-fg-dim truncate mb-1">{{ largeFileConfirm.name }}</p>
        <p class="text-xs text-fg-muted mb-4">
          {{ formatSize(largeFileConfirm.size) }} — files over 3 MB may be slow to open. Continue?
        </p>
        <div class="flex gap-2 justify-end">
          <button @click="largeFileConfirm = null" class="btn-sm btn-ghost">Cancel</button>
          <button @click="loadIntoEditor(largeFileConfirm!); largeFileConfirm = null" class="btn-sm btn-primary">
            Open anyway
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
@reference "../main.css";

.ctx-item {
  @apply block w-full text-left px-3 py-1 text-sm text-fg hover:bg-raised cursor-pointer;
}
</style>

# go-remote-agent — Project Guide

Remote agent management dashboard. Go backend + Vue 3 frontend.

---

## Frontend Design System

Source: `src/main.css` (`@theme` block, Tailwind 4)

### Color Tokens

| Token | Value | Usage |
|-------|-------|-------|
| `primary` | `#4CAF50` | CTA buttons, selection borders, focus rings, progress |
| `primary-hover` | `#43a047` | Hover state on primary buttons |
| `primary-light` | `#81c784` | Link text on dark backgrounds |
| `base` | `#09090b` | App background (`bg-base`) |
| `surface` | `#18181b` | Panels / cards (`bg-surface`) |
| `raised` | `#27272a` | Input fields (`bg-raised`) |
| `control` | `#3f3f46` | Buttons, badges (`bg-control`) |
| `control-hover` | `#52525b` | Hover on control elements |
| `border` | `#3f3f46` | Default borders (`border-border`) |
| `border-strong` | `#52525b` | Input borders (`border-border-strong`) |
| `fg` | `#f4f4f5` | Primary text (`text-fg`) |
| `fg-dim` | `#d4d4d8` | Secondary text (`text-fg-dim`) |
| `fg-muted` | `#a1a1aa` | Labels, section headings (`text-fg-muted`) |
| `fg-subtle` | `#71717a` | Placeholders, hints (`text-fg-subtle`) |
| `danger` | `#dc2626` | Destructive actions, errors |
| `danger-hover` | `#b91c1c` | Hover on danger |
| `warning` | `#f59e0b` | Upgrade badge, warnings |
| `info` | `#2563eb` | In-progress state (file transfer) |

### Component Classes (`@layer components` in `src/main.css`)

Buttons compose a **size** class + **variant** class:

| Size | Class | Padding |
|------|-------|---------|
| Default | `btn` | `px-3 py-1.5 text-sm` |
| Small | `btn-sm` | `px-2 py-1 text-xs` |

| Variant | Class | Use |
|---------|-------|-----|
| Primary | `btn-primary` | Main CTA |
| Ghost | `btn-ghost` | Secondary / default |
| Warning | `btn-warning` | Upgrade / caution |
| Danger | `btn-danger` | Delete / destructive |

Examples: `btn btn-primary`, `btn-sm btn-ghost`, `btn-sm btn-danger`

| Class | Purpose |
|-------|---------|
| `field` | `<input>` / `<textarea>` — dark bg, focus ring, placeholder style |
| `panel` | Card container (`bg-surface border border-border rounded-lg`) |
| `section-title` | Small uppercase section heading |

### Typography

- UI text: system sans-serif (default)
- IDs, addresses, paths, terminal content: `font-mono`

---

## Frontend File Map

| File | Responsibility |
|------|---------------|
| `src/main.css` | Tailwind 4 import + design tokens (`@theme`) |
| `src/App.vue` | Root layout, PTY→router watcher |
| `src/views/HomeView.vue` | Agent list + proxy manager layout |
| `src/views/TerminalView.vue` | Terminal + file transfer layout |
| `src/components/AgentList.vue` | API key form, agent selection list |
| `src/components/ProxyManager.vue` | Proxy CRUD form + table |
| `src/components/UpgradeButton.vue` | Upgrade trigger + streaming log modal |
| `src/components/TerminalContainer.vue` | PTY config form → xterm terminal |
| `src/components/Terminal.vue` | xterm.js mount, WebSocket PTY integration |
| `src/components/FileTransferToolbar.vue` | Upload/download via binary WebSocket protocol |
| `src/components/FileSystemContainer.vue` | Tree-view file browser: expand dirs, double-click to edit, drag-drop upload, right-click menu |
| `src/stores/agent.ts` | Pinia: apiKey, agentInstances, ptyService |
| `src/services/pty.service.ts` | WebSocket + MessagePack PTY protocol |
| `src/services/fs.service.ts` | Chunked binary file transfer + dir listing / delete / mkdir over PTY socket |
| `src/services/proxy.service.ts` | REST client for proxy management API |
| `src/services/upgrade.service.ts` | Streaming upgrade log via fetch |

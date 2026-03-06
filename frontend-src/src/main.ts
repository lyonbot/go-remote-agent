import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import '@xterm/xterm/css/xterm.css'
import './main.css'

Buffer.Uint64BE = class {
  static isUint64BE(): boolean { return false }
}
Buffer.Int64BE = class {
  static isInt64BE(): boolean { return false }
}

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
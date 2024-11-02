Alpine.data('the_app', function () {
  let term
  /** @type {WebSocket} */ let ws

  let app = {
    stage: 1,

    api_key: localStorage.getItem('api_key') || '',
    agent_instances: [],
    agent_id: -1,
    get agent_instance() {
      return this.agent_instances.find(x => x.id == this.agent_id) || {}
    },
    get agent_name() {
      return this.agent_instance?.name
    },
    reload_agent_instances: function () {
      localStorage.setItem('api_key', this.api_key)
      fetch(`./api/agent/`, { headers: { 'X-API-Key': this.api_key } })
        .then(res => res.json())
        .then(data => {
          this.agent_instances = data
        })
        .catch(err => {
          console.error(err)
          this.agent_instances = []
        })
    },

    init() {
      this.agent_id = parseInt(new URLSearchParams(location.search).get('agent_id') || '', 10) || 0
      this.$watch('agent_id', val => {
        const search = new URLSearchParams(location.search)
        search.set('agent_id', val)
        history.replaceState({}, "", `?${search}`)
      })
      this.reload_agent_instances()
    },

    // ----------------------------------------------
    // upgrade

    upgrade_show_dialog: false,
    upgrade_logs: '',
    startUpgrade() {
      this.upgrade_show_dialog = true
      this.upgrade_logs = ''
      this.upgrade_logs += `agent: ${this.agent_name}\n`
      this.upgrade_logs += `agent_id: ${this.agent_id}\n\n`

      // POST /api/agent/:agent_name/upgrade/ and stream response
      fetch(`./api/agent/${this.agent_name}/upgrade/?agent_id=${this.agent_id}`, {
        method: 'POST',
        headers: { 'X-API-Key': this.api_key },
      })
        .then(res => res.body.getReader())
        .then(async reader => {
          const decoder = new TextDecoder()
          while (true) {
            const { value, done } = await reader.read()
            if (done) break

            const chunk = decoder.decode(value)
            this.upgrade_logs += chunk
          }

          this.upgrade_logs += '\n(please reload agent list !!!)'
        })
    },

    // ----------------------------------------------
    // pty

    pty_cmd: 'sh',
    pty_args: '', // lines as args
    pty_env: 'TERM=xterm-256color',
    pty_inherit_env: true,

    // stage 2
    init_stage_2() {
      var { agent_name, agent_id } = this
      var fitAddon = new FitAddon.FitAddon();

      var termContainer = this.$refs.terminal;
      function sendSizeToAgent() {
        const view = new DataView(new ArrayBuffer(4 * 4 + 1));
        view.setUint8(0, 0x03);
        view.setUint16(1, term.cols, true);
        view.setUint16(3, term.rows, true);
        view.setUint16(5, Math.round(termContainer.offsetWidth), true);
        view.setUint16(7, Math.round(termContainer.offsetHeight), true);
        ws.send(new Uint8Array(view.buffer));
      }

      term = new Terminal();
      term.loadAddon(fitAddon);
      term.open(termContainer);
      term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m\r\n')
      term.onResize(sendSizeToAgent)

      window.term = term
      window.addEventListener('resize', debounce(() => fitAddon.fit(), 500))

      const url = `./api/agent/${agent_name}/pty/?api_key=${encodeURIComponent(this.api_key)}`;
      ws = new WebSocket(url);

      ws.onopen = () => {
        term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Connected\r\n`);

        const opts = MessagePack.encode({
          cmd: this.pty_cmd,
          args: this.pty_args ? this.pty_args.split('\n') : [],
          env: this.pty_env ? this.pty_env.split('\n') : [],
          inherit_env: this.pty_inherit_env,
        })
        const buf = new Uint8Array(opts.byteLength + 1)
        buf.set(new TextEncoder().encode('\x01'), 0)
        buf.set(opts, 1)
        ws.send(buf);
      };
      ws.onmessage = async (e) => {
        const data = new Uint8Array(await e.data.arrayBuffer())
        const view = new DataView(data.buffer)
        switch (data[0]) {
          case 0x00:
            term.write(data.slice(1));
            break;
          case 0x01:
            term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Pty Opened\r\n`);
            fitAddon.fit();
            break;
          case 0x02:
            term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m Pty Closed, connection intact.\r\n`);
            break;
          case 0x03:
            // term.write(`\x1B[1;3;33m[${agent_name}]\x1B[0m ${data.slice(1)}\r\n`);
            console.log(new TextDecoder().decode(data.slice(1)));
            break;
          case 0x04: // file chunk written
            resolvePromise(`uploadFileChunk:${view.getBigUint64(1, true)}:${new TextDecoder().decode(data.slice(9))}`)
            break
          case 0x05:
            const info = MessagePack.decode(new Uint8Array(data.slice(1)))
            resolvePromise(`getFileInfo:${info.path}`, info)
            break
          case 0x06: {
            const offset = Number(view.getBigUint64(1, true))
            const length = Number(view.getBigUint64(9, true))
            const path = new TextDecoder().decode(data.slice(17, -length))
            const chunk = new Uint8Array(data.slice(-length))
            resolvePromise(`downloadFileChunk:${path}:${offset}`, { offset: offset, data: chunk })
            break
          }
        }
      };
      ws.onclose = () => {
        term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m Connection Closed\r\n`);
      };

      term.onData((data) => {
        const bufData = new TextEncoder().encode(data);
        const buf = new Uint8Array([0x00, ...bufData]);
        ws.send(buf);
      });
    },

    fs_path: '',
    fs_progress: 0,
    fs_about_dropping_file: false,
    fs_init_progress_bar(el) {
      let timer1;
      this.$watch('fs_progress', val => {
        el.style.width = `${val}%`
        el.style.backgroundColor = val < 100 ? 'blue' : 'green'

        if (timer1) {
          clearTimeout(timer1)
          timer1 = null
        }
        if (val >= 100) {
          timer1 = setTimeout(() => { this.fs_progress = 0 }, 2000)
        }
      })
    },

    fs_validate_path() {
      const path = this.fs_path.trim()
      if (!path) {
        alert('path is empty')
        throw new Error('path is empty')
      }

      return path
    },
    fs_download() {
      let path = this.fs_validate_path()
      fs.downloadFile(path, {
        onprogress: val => this.fs_progress = val
      })
    },
    fs_upload(path, file) {
      if (!file) {
        alert('file is empty')
        return
      }

      const reader = new FileReader()
      reader.onload = async (e) => {
        const data = new Uint8Array(e.target.result)
        await fs.uploadFile(path, data, {
          onprogress: val => this.fs_progress = val
        })
      }
      reader.readAsArrayBuffer(file)
    },
    fs_upload_via_button() {
      const path = this.fs_validate_path()
      const picker = this.$refs.upload_picker
      picker.value = ''
      picker.onchange = () => {
        const file = picker.files[0]
        this.fs_upload(path, file)
      }
      picker.click()
    },
    fs_upload_via_drop(event) {
      if (!this.fs_about_dropping_file) return
      this.fs_about_dropping_file = false

      event.preventDefault()
      const path = (prompt('Upload to Path:', this.fs_path) || '').trim()
      if (!path) return

      this.fs_upload(path, event.dataTransfer.files[0])
    },
  }

  const promiseResolvingCallbacks = {}
  const resolvePromise = (id, data) => {
    const callback = promiseResolvingCallbacks[id]
    if (callback) {
      delete promiseResolvingCallbacks[id]
      callback(data)
    }
  }
  const makePromise = (id) => {
    const promise = new Promise((resolve) => {
      const prev = promiseResolvingCallbacks[id]
      promiseResolvingCallbacks[id] = (val) => {
        prev?.(val)
        resolve(val)
      }
    })
    return promise
  }

  const fs = {
    async getFileInfo(path) {
      const promise = makePromise(`getFileInfo:${path}`)
      ws.send(new TextEncoder().encode(`\x05${path}`));
      return await promise
    },
    /** @returns {Promise<{offset: number, data: Uint8Array}>} */
    async downloadFileChunk(path, offset) {
      const promise = makePromise(`downloadFileChunk:${path}:${offset}`)

      var a = new DataView(new ArrayBuffer(17))
      a.setUint8(0, 0x06)
      a.setBigInt64(1, BigInt(offset), true)
      a.setBigUint64(9, 40960n, true)

      var b = new TextEncoder().encode(path)

      var c = new Uint8Array(a.buffer.byteLength + b.byteLength)
      c.set(new Uint8Array(a.buffer), 0)
      c.set(b, a.buffer.byteLength)

      ws.send(c.buffer);

      return await promise
    },
    async downloadFile(path, { onprogress }) {
      const info = await this.getFileInfo(path)
      console.log('got file info', info)

      const chunks = []
      for (let offset = 0; offset < info.size;) {
        const chunk = await this.downloadFileChunk(path, offset)
        chunks.push(chunk.data)
        offset += chunk.data.length
        if (chunk.data.length === 0) {
          console.error('chunk is empty')
          break
        }

        const percentage = offset / info.size * 100
        console.log('Downloaded ', percentage)
        onprogress?.(percentage)
      }

      const filename = info.path.split('/').pop() || 'download'
      const file = new File(chunks, filename, {
        lastModified: info.mtime * 1000,
        type: 'application/octet-stream'
      })
      const url = URL.createObjectURL(file)

      const link = document.createElement('a')
      link.download = filename
      link.href = url
      link.click()
    },
    async uploadFileChunk(path, offset, chunk) {
      const promise = makePromise(`uploadFileChunk:${offset}:${path}`)

      const pathBytes = new TextEncoder().encode(path)
      const payload = new Uint8Array(chunk.byteLength + 17 + pathBytes.byteLength)
      const view = new DataView(payload.buffer)
      view.setUint8(0, 0x04)
      view.setBigUint64(1, BigInt(offset), true)
      view.setBigUint64(9, BigInt(chunk.byteLength), true)
      payload.set(pathBytes, 17)
      payload.set(chunk, pathBytes.byteLength + 17)

      ws.send(payload.buffer)
      await promise
    },
    async uploadFile(path, data, { onprogress }) {
      // first truncate the file to the length of data, allocating space
      await fs.uploadFileChunk(path, data.length, new Uint8Array(0))

      const chunkSize = 40960
      for (let offset = 0; offset < data.length;) {
        const chunk = data.slice(offset, offset + chunkSize)
        await fs.uploadFileChunk(path, offset, chunk)

        offset += chunk.byteLength

        const percentage = offset / data.length * 100
        console.log('Uploaded ', percentage)
        onprogress?.(percentage)
      }
    },
  }

  return app
})

// a debounce with leading and trailing call
function debounce(func, wait = 500) {
  let timeout = null
  return function () {
    const context = this
    const args = arguments
    const later = function () {
      timeout = null
      func.apply(context, args)
    }
    if (timeout !== null) {
      clearTimeout(timeout)
    } else {
      func.apply(context, args)
    }
    timeout = setTimeout(later, wait)
  }
}

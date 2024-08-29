import { Terminal } from './xterm.min.mjs'
import { encode, decode } from './msgpack.min.mjs'

var term = new Terminal();
term.open(document.getElementById('terminal'));
term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m\r\n')

async function main() {
  let search = new URLSearchParams(location.search)
  let agent_name = prompt("agent name?", search.get('agent') || "");
  if (!agent_name) {
    term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m No agent name\r\n`);
    return
  }

  search.set('agent', agent_name)
  history.replaceState({}, "", `?${search}`)

  const url = `./api/client/${agent_name}/pty/`;
  const ws = new WebSocket(url);

  /** @type {Record<string, (info: FileInfo) => void>} */
  const fileResultResolving = {}
  /** @type {Record<string, (offset: number, data: Uint8Array) => void>} */
  const fileChunkResolving = {}
  async function getFileInfo(path) {
    return new Promise((resolve) => {
      fileResultResolving[path] = resolve
      ws.send(new TextEncoder().encode(`\x04${path}`));
    })
  }
  /** @returns {Promise<{offset: number, data: Uint8Array}>} */
  async function downloadFileChunk(path, offset) {
    return new Promise((resolve) => {
      fileChunkResolving[path] = resolve

      var a = new DataView(new ArrayBuffer(17))
      a.setUint8(0, 0x05)
      a.setBigInt64(1, BigInt(offset), true)
      a.setBigUint64(9, 40960n, true)

      var b = new TextEncoder().encode(path)

      var c = new Uint8Array(a.buffer.byteLength + b.byteLength)
      c.set(new Uint8Array(a.buffer), 0)
      c.set(b, a.buffer.byteLength)

      ws.send(c.buffer);
    })
  }
  async function downloadFile(path) {
    const info = await getFileInfo(path)
    console.log('got file info', info)

    const chunks = []
    for (let offset = 0; offset < info.size;) {
      const chunk = await downloadFileChunk(path, offset)
      chunks.push(chunk.data)
      offset += chunk.data.length
      if (chunk.data.length === 0) {
        console.error('chunk is empty')
        break
      }

      console.log('Downloaded ', offset / info.size * 100)
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
  }
  async function uploadFile(path, data) {
    const chunkSize = 40960
    const pathBytes = new TextEncoder().encode(path)
    for (let offset = 0; offset < data.length;) {
      const chunk = data.slice(offset, offset + chunkSize)
      
      const payload = new Uint8Array(chunk.byteLength + 17 + pathBytes.byteLength)
      const view = new DataView(payload.buffer)
      view.setUint8(0, 0x03)
      view.setBigUint64(1, BigInt(offset), true)
      view.setBigUint64(9, BigInt(chunk.byteLength), true)
      payload.set(pathBytes, 17)
      payload.set(chunk, pathBytes.byteLength + 17)

      ws.send(payload.buffer)

      offset += chunk.byteLength
      console.log('Uploaded ', offset / data.length * 100)
    }
  }

  document.getElementById('download_btn').addEventListener('click', async () => {
    const path = document.getElementById('file_path').value.trim()
    if (!path) {
      alert('path is empty')
      return
    }
    await downloadFile(path)
  })

  document.getElementById('upload_btn').addEventListener('click', async () => {
    const path = document.getElementById('file_path').value.trim()
    if (!path) {
      alert('path is empty')
      return
    }

    const picker = document.getElementById('upload_picker')
    picker.value = ''
    picker.onchange = () => {
      const file = picker.files[0]
      const reader = new FileReader()
      reader.onload = async (e) => {
        const data = new Uint8Array(e.target.result)
        await uploadFile(path, data)
      }
      reader.readAsArrayBuffer(file)
    }
    picker.click()
  })

  ws.onopen = () => {
    term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Connected\r\n`);

    ws.send(new TextEncoder().encode('\x01sh'));
  };

  ws.onmessage = async (e) => {
    let data = new Uint8Array(await e.data.arrayBuffer())
    switch (data[0]) {
      case 0x00:
        term.write(data.slice(1));
        break;
      case 0x01:
        term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Pty Opened\r\n`);
        break;
      case 0x02:
        term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m Pty Closed, connection intact.\r\n`);
        break;
      case 0x03:
        // term.write(`\x1B[1;3;33m[${agent_name}]\x1B[0m ${data.slice(1)}\r\n`);
        console.log(new TextDecoder().decode(data.slice(1)));
        break;
      case 0x04:
        const info = decode(new Uint8Array(data.slice(1)))
        if (fileResultResolving[info.path]) {
          fileResultResolving[info.path](info)
          delete fileResultResolving[info.path]
        }
        break
      case 0x05: {
        const view = new DataView(data.buffer)
        const offset = Number(view.getBigUint64(1, true))
        const length = Number(view.getBigUint64(9, true))
        const path = new TextDecoder().decode(data.slice(17, -length))
        const chunk = new Uint8Array(data.slice(-length))
        if (fileChunkResolving[path]) {
          fileChunkResolving[path]({ offset: offset, data: chunk })
          delete fileChunkResolving[path]
        }
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
}

main()
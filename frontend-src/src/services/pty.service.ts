import * as MessagePack from '@msgpack/msgpack'
import { RecvMessageType, SendMessageType } from './types'

import { FsService } from './fs.service'
import type { Terminal } from '@xterm/xterm'

export interface PtyTermOptions {
  cmd: string
  args?: string[]
  env?: string[]
  inherit_env?: boolean
}

export class PtyService {
  public agentName: string
  public agentId?: number
  public apiKey: string
  public ws: WebSocket | null

  private term: Terminal | null
  private connetcPromise: Promise<void> | null
  private msgHandlers: Array<(data: Uint8Array) => boolean>;

  public fs: FsService

  constructor(apiKey: string, agentName: string, agentId?: number) {
    this.agentName = agentName
    this.agentId = agentId
    this.apiKey = apiKey
    this.ws = null
    this.term = null
    this.connetcPromise = null
    this.msgHandlers = []
    this.connect()

    this.fs = new FsService(this)
  }

  setTerm(term: Terminal | null): void {
    this.term = term
  }

  connect(): Promise<void> {
    this.connetcPromise ||= new Promise((resolve, reject) => {
      var url = `./api/agent/${this.agentName}/omni/?api_key=${encodeURIComponent(this.apiKey)}`
      if (this.agentId) {
        url += `&agent_id=${this.agentId}`
      }
      const ws = new WebSocket(url)
      this.ws = ws
      this.ws.onopen = () => {
        this.term?.write(`\x1B[1;3;32m[${this.agentName}]\x1B[0m Connected\r\n`)
        resolve()
      }
      this.ws.onerror = (err: Event) => {
        reject(err)
        if (this.ws === ws) {
          this.connetcPromise = null
          this.ws = null
        }
      }

      this.ws.onmessage = this.handleMessage.bind(this)
      this.ws.onclose = () => {
        this.term?.write(`\x1B[1;3;31m[${this.agentName}]\x1B[0m Connection Closed\r\n`)
        this.connetcPromise = null
      }
    })
    return this.connetcPromise
  }

  close(): void {
    this.ws?.close()
  }

  createPty(options: PtyTermOptions): void {
    const opts = MessagePack.encode({
      cmd: options.cmd,
      args: options.args || [],
      env: options.env || [],
      inherit_env: options.inherit_env !== false
    })
    const buf = new Uint8Array(opts.byteLength + 1)
    buf.set([SendMessageType.PtyOpen], 0)
    buf.set(opts, 1)
    this.ws?.send(buf)
  }

  private handleMessage = async (e: MessageEvent): Promise<void> => {
    const data = new Uint8Array(await e.data.arrayBuffer())

    switch (data[0]) {
      case RecvMessageType.PtyData:
        this.term?.write(data.slice(1))
        break
      case RecvMessageType.PtyOpened:
        this.term?.write(`\x1B[1;3;32m[${this.agentName}]\x1B[0m Pty Opened\r\n`)
        break
      case RecvMessageType.PtyClosed:
        this.term?.write(`\x1B[1;3;31m[${this.agentName}]\x1B[0m Pty Closed, connection intact.\r\n`)
        break
      case RecvMessageType.Log:
        console.log(new TextDecoder().decode(data.slice(1)))
        break
      default:
        // Handle other messages through callbacks
        for (const handler of this.msgHandlers) {
          if (handler(data)) break
        }
        break
    }
  }

  addMessageHandler(handler: (data: Uint8Array) => boolean): void {
    this.msgHandlers.push(handler)
  }

  resizeTerm(cols: number, rows: number): void {
    const buf = new Uint8Array(1 + 8)
    const bufView = new DataView(buf.buffer)
    bufView.setUint8(0, SendMessageType.PtyResize)
    bufView.setUint16(1, cols, true)
    bufView.setUint16(3, rows, true)
    this.ws?.send(buf)
  }

  sendTermData(data: string): void {
    const bufData = new TextEncoder().encode(data)
    const buf = new Uint8Array([SendMessageType.PtyWrite, ...bufData])
    this.ws?.send(buf)
  }
}
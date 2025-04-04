import { type FileInfo, type DownloadChunkResponse, SendMessageType, RecvMessageType } from './types'
import { PtyService } from './pty.service'
import * as MessagePack from '@msgpack/msgpack'

interface TransferFileOptions {
  onprogress?: (percentage: number) => void
}

/**
 * (do not use this class directly, use `pty.fs` instead)
 */
export class FsService {
  private pty: PtyService
  private promises: Map<string, { resolve: (value: any) => void; reject: (reason?: any) => void }> = new Map()

  constructor(ptyService: PtyService) {
    this.pty = ptyService
    this.pty.addMessageHandler(this.handleMessage.bind(this))
  }

  private makePromise(key: string): Promise<any> {
    return new Promise((resolve, reject) => {
      this.promises.set(key, { resolve, reject })
    })
  }

  private resolvePromise(key: string, value: any = undefined): void {
    const promise = this.promises.get(key)
    if (promise) {
      promise.resolve(value)
      this.promises.delete(key)
    }
  }

  private handleMessage(data: Uint8Array): boolean {
    const view = new DataView(data.buffer)

    switch (data[0]) {
      case RecvMessageType.FileWritten:
        this.handleUploadChunk(view, data)
        break
      case RecvMessageType.FileInfo:
        this.handleFileInfo(data)
        break
      case RecvMessageType.FileChunkRead:
        this.handleDownloadChunk(view, data)
        break
      default:
        return false
    }
    return true
  }

  private handleUploadChunk(view: DataView, data: Uint8Array): void {
    const offset = view.getBigUint64(1, true)
    const path = new TextDecoder().decode(data.slice(9))
    this.resolvePromise(`uploadFileChunk:${offset}:${path}`)
  }

  private handleFileInfo(data: Uint8Array): void {
    const info = MessagePack.decode(new Uint8Array(data.slice(1))) as FileInfo
    this.resolvePromise(`getFileInfo:${info.path}`, info)
  }

  private handleDownloadChunk(view: DataView, data: Uint8Array): void {
    const offset = Number(view.getBigUint64(1, true))
    const length = Number(view.getBigUint64(9, true))
    const path = new TextDecoder().decode(data.slice(17, -length))
    const chunk = new Uint8Array(data.slice(-length))
    this.resolvePromise(`downloadFileChunk:${path}:${offset}`, { offset, data: chunk })
  }

  async getFileInfo(path: string): Promise<FileInfo> {
    const promise = this.makePromise(`getFileInfo:${path}`)
    this.pty.ws?.send(new TextEncoder().encode(`${String.fromCharCode(SendMessageType.FileQueryInfo)}${path}`))
    return await promise
  }

  async downloadFileChunk(path: string, offset: number): Promise<DownloadChunkResponse> {
    const promise = this.makePromise(`downloadFileChunk:${path}:${offset}`)
    const header = new DataView(new ArrayBuffer(17))
    header.setUint8(0, SendMessageType.FileRead)
    header.setBigInt64(1, BigInt(offset), true)
    header.setBigUint64(9, 40960n, true)

    const pathBytes = new TextEncoder().encode(path)
    const payload = new Uint8Array(header.buffer.byteLength + pathBytes.byteLength)
    payload.set(new Uint8Array(header.buffer), 0)
    payload.set(pathBytes, header.buffer.byteLength)

    this.pty.ws?.send(payload.buffer)
    return await promise
  }

  async downloadFile(path: string, options: TransferFileOptions): Promise<void> {
    const info = await this.getFileInfo(path)
    const chunks: Uint8Array[] = []

    for (let offset = 0; offset < info.size;) {
      const chunk = await this.downloadFileChunk(path, offset)
      chunks.push(chunk.data)
      offset += chunk.data.length

      if (chunk.data.length === 0) break

      const percentage = (offset / info.size) * 100
      options.onprogress?.(percentage)
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

  /**
   * upload or truncate a file. if `chunk` is empty, the file will be truncated to `offset` bytes.
   */
  async uploadFileChunk(path: string, offset: number, chunk: Uint8Array): Promise<void> {
    const promise = this.makePromise(`uploadFileChunk:${offset}:${path}`)
    const pathBytes = new TextEncoder().encode(path)
    const payload = new Uint8Array(chunk.byteLength + 17 + pathBytes.byteLength)
    const view = new DataView(payload.buffer)

    view.setUint8(0, SendMessageType.FileWriteOrTruncate)
    view.setBigUint64(1, BigInt(offset), true)
    view.setBigUint64(9, BigInt(chunk.byteLength), true)
    payload.set(pathBytes, 17)
    payload.set(chunk, pathBytes.byteLength + 17)

    this.pty.ws?.send(payload.buffer)
    await promise
  }

  async uploadFile(path: string, data: Uint8Array, options: TransferFileOptions): Promise<void> {
    await this.uploadFileChunk(path, data.length, new Uint8Array(0))

    const chunkSize = 40960
    for (let offset = 0; offset < data.length;) {
      const chunk = data.slice(offset, offset + chunkSize)
      await this.uploadFileChunk(path, offset, chunk)
      offset += chunk.byteLength
      options.onprogress?.((offset / data.length) * 100)
    }
  }
}

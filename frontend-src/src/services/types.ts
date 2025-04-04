export interface FileInfo {
  path: string
  size: number
  mtime: number
  [key: string]: any
}

export interface DownloadChunkResponse {
  offset: number
  data: Uint8Array
}

export enum SendMessageType {
  Log = 0xFF,

  PtyWrite = 0x00,
  PtyOpen = 0x01,
  PtyClose = 0x02,
  PtyResize = 0x03,

  FileWriteOrTruncate = 0x10,
  FileQueryInfo = 0x11,
  FileRead = 0x12,
}

export enum RecvMessageType {
  Log = 0xFF,

  PtyData = 0x00,
  PtyOpened = 0x01,
  PtyClosed = 0x02,

  FileWritten = 0x10,
  FileInfo = 0x11,
  FileChunkRead = 0x12,
}

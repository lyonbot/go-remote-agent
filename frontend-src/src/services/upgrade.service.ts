export class UpgradeService {
  private apiKey: string
  constructor(apiKey: string) {
    this.apiKey = apiKey
  }

  async startUpgrade(agentName: string, agentId: number, onLog: (log: string) => void) {
    const res = await fetch(`./api/agent/${agentName}/upgrade/?agent_id=${agentId}`, {
      method: 'POST',
      headers: { 'X-API-Key': this.apiKey },
    })
    if (!res.ok || !res.body) {
      onLog(`upgrade failed: ${res.status} ${res.statusText}`)
      throw new Error(`upgrade failed: ${res.status} ${res.statusText}`)
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder()

    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      onLog(decoder.decode(value))
    }

    onLog('\n(please reload agent list !!!)')
  }
}
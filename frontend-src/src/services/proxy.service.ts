export interface ProxyDef {
  host: string
  agent_name: string
  agent_id: string
  target: string
  replace_host: string
}

export class ProxyService {
  public apiKey: string

  constructor(apiKey: string) {
    this.apiKey = apiKey
  }

  async loadProxyList(): Promise<ProxyDef[]> {
    const res = await fetch(`./api/proxy/`, {
      headers: { 'X-API-Key': this.apiKey }
    })
    return await res.json()
  }

  async deleteProxy(host: string): Promise<any> {
    const res = await fetch(`./api/proxy/${encodeURIComponent(host)}/`, {
      method: 'DELETE',
      headers: { 'X-API-Key': this.apiKey }
    })
    if (!res.ok) {
      throw new Error(await res.text().catch(() => 'Unknown error'))
    }
    return await res.json()
  }

  async createProxy(data: ProxyDef): Promise<any> {
    const body = new FormData()
    body.append('agent_name', data.agent_name)
    body.append('agent_id', data.agent_id)
    body.append('target', data.target)
    body.append('replace_host', data.replace_host)

    const res = await fetch(`./api/proxy/${encodeURIComponent(data.host)}/`, {
      method: 'POST',
      headers: { 'X-API-Key': this.apiKey },
      body,
    })
    if (!res.ok) {
      throw new Error(await res.text().catch(() => 'Unknown error'))
    }

    return await res.json()
  }
}
# go-remote-agent

A single binary that runs as either a **Server** or an **Agent**, enabling remote shell execution, interactive terminals, file transfer, and HTTP proxying — all without requiring a public IP on the agent side.

```
Client ──HTTP/WS──▶ Server ◀──HTTP polling── Agent (no public IP needed)
```

## Modes

- **Server** — starts an HTTP server; manages agent connections, exposes a REST API and web UI
- **Agent** — connects to the server via HTTP long-polling; receives and executes tasks

## Quick Start

### Build

```sh
make build
```

### Run as Server

```sh
./agent_linux_amd64
# Listening on 0.0.0.0:8080
```

### Run as Agent

```sh
./agent_linux_amd64 -a -n bot1 -b http://your-server:8080
```

### Send a Command

```sh
curl http://localhost:8080/api/agent/bot1/exec/ -F "cmd=uname -a" -F stderr=1
```

## Configuration

`config.yaml` (use `-c path` to override):

```yaml
# Common
name: bot1 # agent name (required for agent mode)

# Server-only
addr: 0.0.0.0
port: 8080
api_key: your_secret # if omitted, all clients are allowed (warning logged)
proxy_server_host: "*.proxy.your-domain.com" # must contain *
proxy_services:
  - host: foobar.proxy.your-domain.com
    agent_name: bot1
    target: http://127.0.0.1:8765
    replace_host: foobar.your-domain.com # optional

# Agent-only
as_agent: true # or use -a flag
base_url: http://server:8080
insecure: false # skip TLS verification
```

### CLI Flags

| Flag             | Description                               |
| ---------------- | ----------------------------------------- |
| `-c <path>`      | Config file path (default: `config.yaml`) |
| `-a`             | Enable agent mode                         |
| `-n <name>`      | Agent / server name                       |
| `-b <url>`       | Base URL (agent mode)                     |
| `-i`             | Insecure TLS (agent mode)                 |
| `-ak <key>`      | API key (server mode)                     |
| `-psh <pattern>` | Proxy server host pattern (server mode)   |

> All string flags support environment variable substitution: `-b '$SERVER_URL'`

## API Key Authentication

Set `api_key` in config (or `-ak`). Clients must provide it via one of:

| Method      | Example                     |
| ----------- | --------------------------- |
| Query param | `?api_key=xxx`              |
| POST field  | `api_key=xxx`               |
| Header      | `X-API-Key: xxx`            |
| Header      | `Authorization: Bearer xxx` |

## REST API

### Agent Management

| Method | Path                         | Description                        |
| ------ | ---------------------------- | ---------------------------------- |
| `GET`  | `/api/agent/`                | List all connected agent instances |
| `GET`  | `/api/agent/{name}/`         | List instances of a named agent    |
| `POST` | `/api/agent/{name}/exec/`    | Execute a shell command            |
| `GET`  | `/api/agent/{name}/omni/`    | Open an omni session (WebSocket)   |
| `POST` | `/api/agent/{name}/upgrade/` | Upgrade agent binary               |

#### POST /api/agent/{name}/exec/

Form fields:

| Field      | Description                                      |
| ---------- | ------------------------------------------------ |
| `cmd`      | Shell command (runs as `sh -c <cmd>`)            |
| `agent_id` | (optional) Target a specific instance            |
| `stdin`    | (optional) File or text piped to stdin           |
| `stdout`   | Set to `0` to suppress stdout (default: enabled) |
| `stderr`   | Set to `1` to enable stderr                      |
| `full`     | Set to `1` for raw binary protocol output        |

Response is a chunked stream. HTTP 200 = success; non-200 body is an error message.

### Proxy Services

| Method   | Path                 | Description             |
| -------- | -------------------- | ----------------------- |
| `GET`    | `/api/proxy/`        | List all proxy services |
| `POST`   | `/api/proxy/{host}/` | Create a proxy service  |
| `DELETE` | `/api/proxy/{host}/` | Remove a proxy service  |

#### POST /api/proxy/{host}/

Form fields: `host`, `agent_name` or `agent_id`, `target`, `replace_host` (optional).

### Config

| Method | Path              | Description                             |
| ------ | ----------------- | --------------------------------------- |
| `POST` | `/api/saveConfig` | Persist current config to `config.yaml` |

## Proxy Host (ngrok-like)

The server can forward HTTP(S)/WebSocket requests to a target service running behind the agent:

```
User → xxx.proxy.your-domain.com → Server → (omni session) → Agent → http://localhost:8765
```

Configure `proxy_server_host: "*.proxy.your-domain.com"` so short names like `foobar` expand to `foobar.proxy.your-domain.com`.

### Nginx Example

```nginx
server {
    listen 80;
    server_name *.proxy.your-domain.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_buffering off;

        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }
}
```

### Apache Example

```apache
<VirtualHost *:443>
  ServerName x.proxy.your-domain.com
  ServerAlias *.proxy.your-domain.com

  RewriteEngine On
  RewriteCond %{HTTP:Upgrade} =websocket [NC]
  RewriteRule /(.*)  ws://127.0.0.1:8080/$1 [P,L]
  RewriteCond %{HTTP:Upgrade} !=websocket [NC]
  RewriteRule /(.*)  http://127.0.0.1:8080/$1 [P,L]
  ProxyPreserveHost On
</VirtualHost>
```

## Web UI

The server serves a built-in Vue 3 web UI at `/`. It supports:

- Viewing connected agents and upgrade status
- Interactive terminal (xterm.js)
- File upload/download
- Proxy service management

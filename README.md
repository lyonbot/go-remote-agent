# Remote Agent

This is a program that can act as a server or a agent.

When act as an `Agent`, it automatically connects to server, wait and run commands from server.

- Agent doesn't need a public IP. It connects to server via http/https.

When act as a `Server`. It starts a HTTP server...

- **For Agents**: push commands to agents, and create tunnels for further communication (like shell, proxy, file transfer, etc.).
- **For API clients**: so you can interact with agents.
- **Act as Proxy Host**: works like ngrok server. See [Proxy Host](#proxy-host) section.

## Usage

```sh
export GOARCH=amd64 GOOS=linux
go build -o agent_linux_amd64 -v .
go build -ldflags "-s -w" -tags release -o agent_linux_amd64 -v .
```

Then run it:

```sh
# run as a server
./agent_linux_amd64

# run as an agent
./agent_linux_amd64 -a -n bot1 -b http://127.0.0.1:8080

# send command to agent
curl http://127.0.0.1:8080/api/agent/bot1/exec/ -F "cmd=ffmpeg -codecs" -F stderr=1
curl http://127.0.0.1:8080/api/agent/bot1/exec/ -F "cmd=wc -c" -F stdin=@file.bin
```

## API

The `Server` provides these APIs.

### GET /api/agent/

List all agent instances. Returns a JSON array like :

```json
[
  {
    "id": 1,
    "name": "bot1",
    "user_agent": "go-remote-agent/a7c3f99532177ddd8157aa7b640c2622b15af9c2@1725035270 (darwin; amd64)",
    "is_upgradable": false,
    "join_at": "2024-08-31T00:42:44.694602+08:00",
    "remote_addr": "[::1]:64922"
  }
]
```

### GET /api/agent/:agent_name/

List all agent instances of given name. Same response as above.

### POST /api/agent/:agent_name/exec/

Run a command on the agent. The payload is a FormData:

- `agent_id`: (optional) agent instance id, must match the agent name
- `cmd`: string, command to run. will be passed as `sh -c <cmd>`
- `stdin`: (optional) file
- `stdout`: (optional) set to `0` to disable stdout (default enabled)
- `stderr`: (optional) set to `1` to enable stderr
- `full`: (optional) set to `1` to recv all raw data:
  - `<uint32 length> <package>` where `<package>` is same as A->S above, length is littleEndian
  - meanwhile, `stdout` and `stderr` will be disabled

It streams the response to the client.

Note: check status must be 200, otherwise the body is error message.

### GET /api/agent/:agent_name/omni/

Open an "omni" session. This is a WebSocket connection.

You can put query parameters:

- `agent_id`: (optional) agent instance id, must match the agent name

The communication protocol is same as ["omni" session protocol](#omni-session-protocol).

### POST /api/agent/:agent_name/upgrade/

Upgrade the agent to a new executable, matching the one that server is running.

You can put query parameters:

- `agent_id`: (optional) agent instance id, must match the agent name

### GET /api/proxy/

List all proxy channels.

### POST /api/proxy/:channel_id/

Create a proxy channel. The payload is a FormData:

- `agent_id` or `agent_name`: agent instance id or name
- `target`: address like `http://127.0.0.1:8080`

### DELETE /api/proxy/:channel_id/

Stop a proxy channel.

## API Key

To secure your `Server`, you can set an `api_key` in the config file, or via `-ak your_key` command line.

API client may pass API key in one of these ways:

- Query parameter: `api_key`
- POST field: `api_key`
- Header: `X-API-Key`
- Header: `Authorization: Bearer <api_key>`

## Proxy Host

(Only works on `Server`)

You can provide a host pattern like `*.proxy.my-site.com` via `proxy_server_host` in the config file, or via `-psh` command line.

## "omni" session protocol

In a "omni" session, you can:

- start a pty shell session on agent.
- upload / download files.
- proxy TCP connections via agent.

### common features

A->S:

- `0xff <message_str>` - debug message

### pty feature

You can start up to 1 pty shell per "omni" session.

S->A:

- `0x00 <data>` - pty data
- `0x01 <msgpack>` - start a pty session. msgpack is optional, may contains these fields:
  - `cmd`: string, command to run. default is `sh`
  - `args`: string[] like `["arg1", "arg2"]`
  - `env`: string[] like `["FOO=bar", "BAR=foo"]`
  - `no_inherit_env`: bool, if true, don't inherit environment variables from parent process
- `0x02` - close pty
- `0x03 <uint16 cols> <uint16 rows> <uint16 width> <uint16 height>` - resize pty

A->S:

- `0x00 <data>` - pty data
- `0x01` - pty opened
- `0x02` - pty closed

### file transfer feature

S->A:

- `0x10 <uint64 offset> <uint64 length> <file_path> <data>` - write a file chunk / truncate a file to length of _offset_ (if _data_ is empty)
- `0x11 <file_path>` - read file info
- `0x12 <uint64 offset> <uint64 length> <file_path>` - request to read a file chunk

A->S:

- `0x10 <uint64 offset> <file_path>` - file chunk written / file truncated
- `0x11 <msgpack FileInfo>` - queried file info, see protocol.go
- `0x12 <uint64 offset> <uint64 length> <file_path> <data>` - read a file chunk

### proxy feature

S->A:

- `0x20 <uint32 id> <uint16 port> <string addr>` - open a TCP proxy channel `id`. connect to `addr:port`.
- `0x21 <uint32 id> <data>` - send data to proxy channel `id`.
- `0x22 <uint32 id>` - close proxy channel `id`.

A->S:

- `0x20 <uint32 id> <uint8 code> <string errmsg>` - proxy channel opened or not. code = 0 means success
- `0x21 <uint32 id> <data>` - proxy channel data.
- `0x22 <uint32 id>` - proxy channel closed.

## How Agent communicate with Server

(You may skip this part -- it's for developers only)

Agents also talk to server in HTTP protocol, but based on the User-Agent header, they have different API entrypoints.

When client request to run a command, the server will push a `AgentNotify` message with a `token`, to the agent.
(Pushed via `/api/agent/:agent_name`)

The agent will then connect to the server via WebSocket, with the `token`, then execute the command.
(WebSocket connection via `/api/agent/:agent_name/:token`)

Here is the list of agent-only APIs:

### GET /api/agent/:agent_name

For agent.

This is a binary stream. When a client want to run a command, the server will push the command to the agent.

Each package starts with a `uint32` of length (in LittleEndian), then the data of `AgentNotify` (encoded by `msgp`), then a CRLF (`0x0d 0x0a`).

### GET /api/agent/:agent_name/:token

For agent.

This is a WebSocket connection.

#### shell session

S->A:

- `0x00 <data>` - write stdin
- `0x01` - close stdin
- `0x02 <int32_signal>` - send signal

A->S

- `0x00 <int32_code>` - exit code
- `0x01 <data>` - stdout
- `0x02 <data>` - stderr
- `0x03 <message_str>` - debug message

#### "omni" session

See ["omni" session protocol](#omni-session-protocol).

#### version upgrade session

In version upgrade session, the server will send the binary of agent executable, and agent will try to upgrade itself.

Note: in each step, agent may send `0x99 <error_message>` to server, and server will terminate the connection.

1. S->A: `0x00` -- check whether upgradable. sometimes agent cannot rename itself.
2. A->S: `0x00 <executable_path>` -- continue
3. S->A: `0x01 [int64 total_size]` -- send agent executable info
4. (Repeat) S->A: `0x02 [int64 offset] [data]` & A->S `0x00 [int64 new_offset]` -- send agent executable chunk
5. A->S: `0x01` -- executable renamed
6. A->S: `0x02` -- new executable started, we're running now

# Develop

## Generate `biz/protocol.go`

```
~/go/bin/msgp -file biz/protocol.go
```

## Locally Test

First, create `config.yaml` like this:

```yaml
# agent: true  # with -a to toggle
name: bot1
base_url: http://localhost:8080
```

Then open three terminals:

- Server: `go run main.go`
- Agent: `go run main.go -a`
- Client: (see below)

```sh
curl http://127.0.0.1:8080/api/client/bot1/exec/ -F "cmd=ffmpeg -codecs" -F stdout=1
curl http://127.0.0.1:8080/api/client/bot1/exec/ -F "cmd=wc -c" -F stdin=@main.go -F stdout=1

# event stream format
```

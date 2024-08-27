package main

import (
	"remote-agent/agent"
	"remote-agent/biz"
	"remote-agent/server"
)

func main() {
	if biz.Config.AsAgent {
		agent.RunAgent()
	} else {
		server.RunServer()
	}
}

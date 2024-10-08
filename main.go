package main

import (
	"remote-agent/agent"
	"remote-agent/biz"
	"remote-agent/server"
)

func main() {
	biz.InitConfig()
	if biz.Config.AsAgent {
		agent.RunAgent()
	} else {
		server.RunServer()
	}
}

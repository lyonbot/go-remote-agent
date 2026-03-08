package main

import (
	"remote-agent/agent"
	"remote-agent/biz"
	"remote-agent/client"
	"remote-agent/server"
)

func main() {
	biz.InitConfig()
	if biz.Config.AsClient {
		client.Run()
	} else if biz.Config.AsAgent {
		agent.RunAgent()
	} else {
		server.RunServer()
	}
}

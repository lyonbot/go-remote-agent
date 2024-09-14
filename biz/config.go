package biz

import (
	"flag"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	Name    string // agent name (server name or agent name)
	AsAgent bool   `yaml:"as_agent"` // true for agent

	// for server
	Addr   string // listen address, defaults to "0.0.0.0"
	Port   int32  // listen port, defaults to 8080
	APIKey string `yaml:"api_key"` // API key for client API. If set, must provided via `X-API-Key` header or `Authorization: Bearer <api_key>` header

	// for agent
	BaseUrl  string `yaml:"base_url"` // base url, including protocol and port, without `/api`
	Insecure bool
}

var Config AgentConfig

func InitConfig() {
	configPath := flag.String("c", "config.yaml", "Config path")
	asAgent := flag.Bool("a", false, "Set agent mode")
	name := flag.String("n", "", "Agent name")
	baseUrl := flag.String("b", "", "Base URL (only for agent)")
	insecure := flag.Bool("i", false, "Insecure (only for agent)")
	api_key := flag.String("ak", "", "API key (only for server)")
	flag.Parse()

	if data, err := os.ReadFile(*configPath); err == nil {
		err = yaml.Unmarshal([]byte(data), &Config)
		if err != nil {
			log.Fatalf("Failed to parse config file %s: %v", *configPath, err)
			panic(err)
		}
	}

	if *asAgent {
		Config.AsAgent = *asAgent
	}
	if *name != "" {
		Config.Name = *name
	}
	if *api_key != "" {
		Config.APIKey = *api_key
	}

	// defaults
	if Config.AsAgent {
		if *baseUrl != "" {
			Config.BaseUrl = *baseUrl
		}
		if *insecure {
			Config.Insecure = true
		}

		if Config.Name == "" {
			log.Fatalf("Name is required")
		}
		if Config.BaseUrl == "" {
			log.Fatalf("BaseUrl is required for agent")
		}
	} else {
		if Config.Addr == "" {
			Config.Addr = "0.0.0.0"
		}
		if Config.Port == 0 {
			Config.Port = 8080
		}
	}
}

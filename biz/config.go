package biz

import (
	"flag"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	Name    string // agent name (server name or agent name)
	AsAgent bool   `yaml:"as_agent"` // true for agent

	// for server
	Addr            string             // listen address, defaults to "0.0.0.0"
	Port            int32              // listen port, defaults to 8080
	APIKey          string             `yaml:"api_key"`           // API key for client API. If set, must provided via `X-API-Key` header or `Authorization: Bearer <api_key>` header
	ProxyServerHost string             `yaml:"proxy_server_host"` // like `foo-*.your-domain.com`. must contain `*`
	ProxyServices   []SavedProxyConfig `yaml:"proxy_services"`

	// for agent
	BaseUrl  string `yaml:"base_url"` // base url, including protocol and port, without `/api`
	Insecure bool
}

type SavedProxyConfig struct {
	Host      string `yaml:"host"`
	AgentName string `yaml:"agent_name"`
	// AgentId   string `yaml:"agent_id"`		// not supported -- id may change
	Target      string `yaml:"target"`
	ReplaceHost string `yaml:"replace_host"`
}

var Config AgentConfig

func maybeEnv(s string) string {
	if strings.HasPrefix(s, "$") {
		return os.Getenv(s[1:])
	}
	return s
}

func InitConfig() {
	configPath := flag.String("c", "config.yaml", "Config path")
	asAgent := flag.Bool("a", false, "Set agent mode")
	name := flag.String("n", "", "Agent name")
	baseUrl := flag.String("b", "", "Base URL (only for agent)")
	insecure := flag.Bool("i", false, "Insecure (only for agent)")
	api_key := flag.String("ak", "", "API key (only for server)")
	proxy_server_host := flag.String("psh", "", "Proxy server host (only for server, must contains *)")
	flag.Parse()

	if data, err := os.ReadFile(maybeEnv(*configPath)); err == nil {
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
		Config.Name = maybeEnv(*name)
	}
	if *api_key != "" {
		Config.APIKey = maybeEnv(*api_key)
	}
	if *proxy_server_host != "" {
		Config.ProxyServerHost = maybeEnv(*proxy_server_host)
	}

	// defaults
	if Config.AsAgent {
		if *baseUrl != "" {
			Config.BaseUrl = maybeEnv(*baseUrl)
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
		if Config.APIKey == "" {
			log.Println("[!] APIKey not set, any client can access agents!")
		}
		if Config.ProxyServerHost != "" && !strings.Contains(Config.ProxyServerHost, "*") {
			log.Fatalf("ProxyServerHost must contains *")
		}
	}
}

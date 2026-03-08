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

	// for client (port forwarding CLI)
	AsClient       bool     `yaml:"as_client"`
	ClientForwards []string `yaml:"-"` // command-line only: localPort:remoteAddr:remotePort
}

// multiFlag allows a flag to be specified multiple times
type MultiFlag []string

func (f *MultiFlag) String() string { return strings.Join(*f, ", ") }
func (f *MultiFlag) Set(v string) error { *f = append(*f, v); return nil }

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

var currentConfigPath = "config.yaml"

func WriteConfigFile() error {
	data, err := yaml.Marshal(&Config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(currentConfigPath, data, 0644); err != nil {
		return err
	}

	return nil
}

func InitConfig() {
	configPath := flag.String("c", "config.yaml", "Config path")
	asAgent := flag.Bool("a", false, "Set agent mode")
	asClient := flag.Bool("client", false, "Set client mode (TCP port forwarding)")
	name := flag.String("n", "", "Agent name")
	baseUrl := flag.String("b", "", "Base URL (for agent or client)")
	insecure := flag.Bool("i", false, "Insecure TLS (for agent or client)")
	api_key := flag.String("ak", "", "API key")
	proxy_server_host := flag.String("psh", "", "Proxy server host (only for server, must contains *)")
	var clientForwards MultiFlag
	flag.Var(&clientForwards, "L", "Port forward (client mode): localPort:remoteAddr:remotePort (repeatable)")
	flag.Parse()

	if data, err := os.ReadFile(maybeEnv(*configPath)); err == nil {
		err = yaml.Unmarshal([]byte(data), &Config)
		if err != nil {
			log.Fatalf("Failed to parse config file %s: %v", *configPath, err)
			panic(err)
		}
	}
	currentConfigPath = *configPath

	if *asAgent {
		Config.AsAgent = *asAgent
	}
	if *asClient {
		Config.AsClient = *asClient
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
	if len(clientForwards) > 0 {
		Config.ClientForwards = clientForwards
	}

	// defaults
	if Config.AsClient {
		if *baseUrl != "" {
			Config.BaseUrl = maybeEnv(*baseUrl)
		}
		if *insecure {
			Config.Insecure = true
		}
		if Config.Name == "" {
			log.Fatalf("Agent name (-n) is required for client mode")
		}
		if Config.BaseUrl == "" {
			log.Fatalf("Server URL (-b) is required for client mode")
		}
		if len(Config.ClientForwards) == 0 {
			log.Fatalf("At least one port forward (-L localPort:remoteAddr:remotePort) is required")
		}
	} else if Config.AsAgent {
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

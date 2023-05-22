package config

import (
	"fmt"
	"github.com/Bitrue-exchange/libada-go"
	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
	"os"
)

type Config struct {
	Logging LoggingConfig `yaml:"logging"`
	Api     ApiConfig     `yaml:"api"`
	Metrics MetricsConfig `yaml:"metrics"`
	Debug   DebugConfig   `yaml:"debug"`
	Node    NodeConfig    `yaml:"node"`
}

type LoggingConfig struct {
	Healthchecks bool   `yaml:"healthchecks" envconfig:"LOGGING_HEALTHCHECKS"`
	Level        string `yaml:"level" envconfig:"LOGGING_LEVEL"`
}

type ApiConfig struct {
	ListenAddress string `yaml:"address" envconfig:"API_LISTEN_ADDRESS"`
	ListenPort    uint   `yaml:"port" envconfig:"API_LISTEN_PORT"`
}

type DebugConfig struct {
	ListenAddress string `yaml:"address" envconfig:"DEBUG_ADDRESS"`
	ListenPort    uint   `yaml:"port" envconfig:"DEBUG_PORT"`
}

type MetricsConfig struct {
	ListenAddress string `yaml:"address" envconfig:"METRICS_LISTEN_ADDRESS"`
	ListenPort    uint   `yaml:"port" envconfig:"METRICS_LISTEN_PORT"`
}

type NodeConfig struct {
	Network      string `yaml:"network" envconfig:"CARDANO_NETWORK"`
	NetworkMagic uint32 `yaml:"networkMagic" envconfig:"CARDANO_NODE_NETWORK_MAGIC"`
	Address      string `yaml:"address" envconfig:"CARDANO_NODE_SOCKET_TCP_HOST"`
	Port         uint   `yaml:"port" envconfig:"CARDANO_NODE_SOCKET_TCP_PORT"`
	SocketPath   string `yaml:"socketPath" envconfig:"CARDANO_NODE_SOCKET_PATH"`
	Timeout      uint   `yaml:"timeout" envconfig:"CARDANO_NODE_SOCKET_TIMEOUT"`
}

// Singleton config instance with default values
var globalConfig = &Config{
	Logging: LoggingConfig{
		Level:        "info",
		Healthchecks: false,
	},
	Api: ApiConfig{
		ListenAddress: "",
		ListenPort:    8090,
	},
	Debug: DebugConfig{
		ListenAddress: "localhost",
		ListenPort:    0,
	},
	Metrics: MetricsConfig{
		ListenAddress: "",
		ListenPort:    8081,
	},
	Node: NodeConfig{
		Network:    "mainnet",
		SocketPath: "/node-ipc/node.socket",
		Timeout:    30,
	},
}

func Load(configFile string) (*Config, error) {
	// Load config file as YAML if provided
	if configFile != "" {
		buf, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("error reading config file: %s", err)
		}
		err = yaml.Unmarshal(buf, globalConfig)
		if err != nil {
			return nil, fmt.Errorf("error parsing config file: %s", err)
		}
	}
	// Load config values from environment variables
	// We use "dummy" as the app name here to (mostly) prevent picking up env
	// vars that we hadn't explicitly specified in annotations above
	err := envconfig.Process("dummy", globalConfig)
	if err != nil {
		return nil, fmt.Errorf("error processing environment: %s", err)
	}
	if err := globalConfig.populateNetworkMagic(); err != nil {
		return nil, err
	}
	if err := globalConfig.checkNode(); err != nil {
		return nil, err
	}
	return globalConfig, nil
}

// Return global config instance
func GetConfig() *Config {
	return globalConfig
}

func (c *Config) populateNetworkMagic() error {
	if c.Node.Network != "" {
		switch c.Node.Network {
		case "preview":
			c.Node.NetworkMagic = libada.Preview.ProtocolMagic()
		case "preprod":
			c.Node.NetworkMagic = libada.Preprod.ProtocolMagic()
		case "testnet":
			c.Node.NetworkMagic = libada.Testnet.ProtocolMagic()
		case "mainnet":
			c.Node.NetworkMagic = libada.Mainnet.ProtocolMagic()
		default:
			return fmt.Errorf("unknown network: %s", c.Node.Network)
		}
	}
	return nil
}

func (c *Config) checkNode() error {
	// Connect to cardano-node
	oConn, err := ouroboros.NewConnection(
		ouroboros.WithNetworkMagic(uint32(c.Node.NetworkMagic)),
		ouroboros.WithNodeToNode(false),
	)
	if err != nil {
		return fmt.Errorf("failure creating Ouroboros connection: %s", err)
	}

	if c.Node.Address != "" && c.Node.Port > 0 {
		// Connect to TCP port
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", c.Node.Address, c.Node.Port)); err != nil {
			return fmt.Errorf("failure connecting to node via TCP: %s", err)
		}
	} else if c.Node.SocketPath != "" {
		// Check that node socket path exists
		if _, err := os.Stat(c.Node.SocketPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("node socket path does not exist: %s", c.Node.SocketPath)
			} else {
				return fmt.Errorf("unknown error checking if node socket path exists: %s", err)
			}
		}
		if err := oConn.Dial("unix", c.Node.SocketPath); err != nil {
			return fmt.Errorf("failure connecting to node via UNIX socket: %s", err)
		}
	} else {
		return fmt.Errorf("you must specify either the UNIX socket path or the address/port for your cardano-node")
	}
	// Close Ouroboros connection
	oConn.Close()
	return nil
}

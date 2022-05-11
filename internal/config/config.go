package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

const (
	TESTNET_MAGIC = 1097911063
	MAINNET_MAGIC = 764824073
)

type Config struct {
	Logging LoggingConfig `yaml:"logging"`
	Api     ApiConfig     `yaml:"api"`
	Metrics MetricsConfig `yaml:"metrics"`
	Debug   DebugConfig   `yaml:"debug"`
	Node    NodeConfig    `yaml:"node"`
}

type LoggingConfig struct {
	Level string `yaml:"level" envconfig:"LOGGING_LEVEL"`
}

type ApiConfig struct {
	ListenAddress  string `yaml:"address" envconfig:"API_LISTEN_ADDRESS"`
	ListenPort     uint   `yaml:"port" envconfig:"API_LISTEN_PORT"`
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
	Network      string `yaml:"network" envconfig:"NETWORK"`
	NetworkMagic uint32 `yaml:"networkMagic" envconfig:"CARDANO_NODE_NETWORK_MAGIC"`
	Address      string `yaml:"address" envconfig:"CARDANO_NODE_ADDRESS"`
	Port         uint   `yaml:"port" envconfig:"CARDANO_NODE_PORT"`
	SocketPath   string `yaml:"socketPath" envconfig:"CARDANO_NODE_SOCKET_PATH"`
}

// Singleton config instance with default values
var globalConfig = &Config{
	Logging: LoggingConfig{
		Level: "info",
	},
	Api: ApiConfig{
		ListenAddress:  "",
		ListenPort:     8090,
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
	},
}

func Load(configFile string) (*Config, error) {
	// Load config file as YAML if provided
	if configFile != "" {
		buf, err := ioutil.ReadFile(configFile)
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
		case "testnet":
			c.Node.NetworkMagic = TESTNET_MAGIC
		case "mainnet":
			c.Node.NetworkMagic = MAINNET_MAGIC
		default:
			return fmt.Errorf("unknown network: %s", c.Node.Network)
		}
	}
	return nil
}

func (c *Config) checkNode() error {
	if c.Node.Address != "" && c.Node.Port > 0 {
		// TODO: add some validation for node address/port
	} else if c.Node.SocketPath != "" {
		// Check that node socket path exists
		if _, err := os.Stat(c.Node.SocketPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("node socket path does not exist: %s", c.Node.SocketPath)
			} else {
				return fmt.Errorf("unknown error checking if node socket path exists: %s", err)
			}
		}
	} else {
		return fmt.Errorf("you must specify either the UNIX socket path or the address/port for your cardano-node")
	}
	return nil
}

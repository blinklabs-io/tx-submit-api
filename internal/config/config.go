// Copyright 2023 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Logging LoggingConfig `yaml:"logging"`
	Api     ApiConfig     `yaml:"api"`
	Metrics MetricsConfig `yaml:"metrics"`
	Debug   DebugConfig   `yaml:"debug"`
	Node    NodeConfig    `yaml:"node"`
	Tls     TlsConfig     `yaml:"tls"`
}

type LoggingConfig struct {
	Healthchecks bool   `yaml:"healthchecks" envconfig:"LOGGING_HEALTHCHECKS"`
	Level        string `yaml:"level"        envconfig:"LOGGING_LEVEL"`
}

type ApiConfig struct {
	ListenAddress string `yaml:"address" envconfig:"API_LISTEN_ADDRESS"`
	ListenPort    uint   `yaml:"port"    envconfig:"API_LISTEN_PORT"`
}

type DebugConfig struct {
	ListenAddress string `yaml:"address" envconfig:"DEBUG_ADDRESS"`
	ListenPort    uint   `yaml:"port"    envconfig:"DEBUG_PORT"`
}

type MetricsConfig struct {
	ListenAddress string `yaml:"address" envconfig:"METRICS_LISTEN_ADDRESS"`
	ListenPort    uint   `yaml:"port"    envconfig:"METRICS_LISTEN_PORT"`
}

type NodeConfig struct {
	Network      string `yaml:"network"      envconfig:"CARDANO_NETWORK"`
	NetworkMagic uint32 `yaml:"networkMagic" envconfig:"CARDANO_NODE_NETWORK_MAGIC"`
	Address      string `yaml:"address"      envconfig:"CARDANO_NODE_SOCKET_TCP_HOST"`
	Port         uint   `yaml:"port"         envconfig:"CARDANO_NODE_SOCKET_TCP_PORT"`
	SkipCheck    bool   `yaml:"skipCheck"    envconfig:"CARDANO_NODE_SKIP_CHECK"`
	SocketPath   string `yaml:"socketPath"   envconfig:"CARDANO_NODE_SOCKET_PATH"`
	Timeout      uint   `yaml:"timeout"      envconfig:"CARDANO_NODE_SOCKET_TIMEOUT"`
}

type TlsConfig struct {
	CertFilePath string `yaml:"certFilePath" envconfig:"TLS_CERT_FILE_PATH"`
	KeyFilePath  string `yaml:"keyFilePath"  envconfig:"TLS_KEY_FILE_PATH"`
}

// Singleton config instance with default values
var globalConfig = &Config{
	Logging: LoggingConfig{
		Level:        "info",
		Healthchecks: false,
	},
	Api: ApiConfig{
		ListenAddress: "0.0.0.0",
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
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		err = yaml.Unmarshal(buf, globalConfig)
		if err != nil {
			return nil, fmt.Errorf("error parsing config file: %w", err)
		}
	}
	// Load config values from environment variables
	// We use "dummy" as the app name here to (mostly) prevent picking up env
	// vars that we hadn't explicitly specified in annotations above
	err := envconfig.Process("dummy", globalConfig)
	if err != nil {
		return nil, fmt.Errorf("error processing environment: %w", err)
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
	if c.Node.NetworkMagic == 0 {
		if c.Node.Network != "" {
			network, ok := ouroboros.NetworkByName(c.Node.Network)
			if !ok {
				return fmt.Errorf("unknown network: %s", c.Node.Network)
			}
			c.Node.NetworkMagic = uint32(network.NetworkMagic)
			return nil
		}
	}
	return nil
}

func (c *Config) checkNode() error {
	if c.Node.SkipCheck {
		return nil
	}
	// Connect to cardano-node
	oConn, err := ouroboros.NewConnection(
		ouroboros.WithNetworkMagic(uint32(c.Node.NetworkMagic)),
		ouroboros.WithNodeToNode(false),
	)
	if err != nil {
		return fmt.Errorf("failure creating Ouroboros connection: %w", err)
	}

	if c.Node.Address != "" && c.Node.Port > 0 {
		// Connect to TCP port
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", c.Node.Address, c.Node.Port)); err != nil {
			return fmt.Errorf("failure connecting to node via TCP: %w", err)
		}
	} else if c.Node.SocketPath != "" {
		// Check that node socket path exists
		if _, err := os.Stat(c.Node.SocketPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("node socket path does not exist: %s", c.Node.SocketPath)
			} else {
				return fmt.Errorf("unknown error checking if node socket path exists: %w", err)
			}
		}
		if err := oConn.Dial("unix", c.Node.SocketPath); err != nil {
			return fmt.Errorf("failure connecting to node via UNIX socket: %w", err)
		}
	} else {
		return fmt.Errorf("you must specify either the UNIX socket path or the address/port for your cardano-node")
	}
	// Close Ouroboros connection
	oConn.Close()
	return nil
}

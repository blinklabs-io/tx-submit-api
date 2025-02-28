// Copyright 2025 Blink Labs Software
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

package submit

import (
	"errors"
	"fmt"
	"math"
	"time"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/localtxsubmission"
)

type Config struct {
	ErrorChan    chan error
	Network      string
	NetworkMagic uint32
	NodeAddress  string
	NodePort     uint
	SocketPath   string
	Timeout      uint
}

func SubmitTx(cfg *Config, txRawBytes []byte) (string, error) {
	// Fail fast if timeout is too large
	if cfg.Timeout > math.MaxInt64 {
		return "", errors.New("given timeout too large")
	}
	// Determine transaction type (era)
	txType, err := ledger.DetermineTransactionType(txRawBytes)
	if err != nil {
		return "", fmt.Errorf(
			"could not parse transaction to determine type: %w",
			err,
		)
	}
	tx, err := ledger.NewTransactionFromCbor(txType, txRawBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse transaction CBOR: %w", err)
	}

	err = cfg.populateNetworkMagic()
	if err != nil {
		return "", fmt.Errorf("failed to populate networkMagic: %w", err)
	}

	// Connect to cardano-node and submit TX using Ouroboros LocalTxSubmission
	oConn, err := ouroboros.NewConnection(
		ouroboros.WithNetworkMagic(uint32(cfg.NetworkMagic)),
		ouroboros.WithErrorChan(cfg.ErrorChan),
		ouroboros.WithNodeToNode(false),
		ouroboros.WithLocalTxSubmissionConfig(
			localtxsubmission.NewConfig(
				localtxsubmission.WithTimeout(
					time.Duration(cfg.Timeout)*time.Second,
				),
			),
		),
	)
	if err != nil {
		return "", fmt.Errorf("failure creating Ouroboros connection: %w", err)
	}
	if cfg.NodeAddress != "" && cfg.NodePort > 0 {
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", cfg.NodeAddress, cfg.NodePort)); err != nil {
			return "", fmt.Errorf("failure connecting to node via TCP: %w", err)
		}
	} else {
		if err := oConn.Dial("unix", cfg.SocketPath); err != nil {
			return "", fmt.Errorf("failure connecting to node via UNIX socket: %w", err)
		}
	}
	defer func() {
		// Close Ouroboros connection
		oConn.Close()
	}()
	// Submit the transaction
	// #nosec G115
	if err := oConn.LocalTxSubmission().Client.SubmitTx(uint16(txType), txRawBytes); err != nil {
		return "", fmt.Errorf("%s", err.Error())
	}
	return tx.Hash(), nil
}

func (c *Config) populateNetworkMagic() error {
	if c.NetworkMagic == 0 {
		if c.Network != "" {
			network, ok := ouroboros.NetworkByName(c.Network)
			if !ok {
				return fmt.Errorf("unknown network: %s", c.Network)
			}
			c.NetworkMagic = uint32(network.NetworkMagic)
			return nil
		}
	}
	return nil
}

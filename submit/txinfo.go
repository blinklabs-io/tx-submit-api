// Copyright 2026 Blink Labs Software
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
	"github.com/blinklabs-io/gouroboros/ledger"
)

// TxInfo holds content signals extracted from raw transaction CBOR.
type TxInfo struct {
	// ScriptType is the highest Plutus version present in the witness set, or
	// "native" for native scripts only, or "none" if no scripts are present.
	// Values: "none", "native", "plutus_v1", "plutus_v2", "plutus_v3".
	ScriptType string

	// HasMinting is true when the transaction mints or burns native tokens.
	HasMinting bool

	// HasReferenceInputs is true when the transaction includes reference inputs
	// (Babbage / Conway feature used heavily by DeFi protocols).
	HasReferenceInputs bool
}

// ParseTxInfo parses raw transaction CBOR and returns content signals without
// submitting the transaction. Returns an error if the bytes cannot be decoded
// as a known Cardano transaction type.
func ParseTxInfo(rawBytes []byte) (*TxInfo, error) {
	txType, err := ledger.DetermineTransactionType(rawBytes)
	if err != nil {
		return nil, err
	}
	tx, err := ledger.NewTransactionFromCbor(txType, rawBytes)
	if err != nil {
		return nil, err
	}

	info := &TxInfo{
		HasMinting:         tx.AssetMint() != nil,
		HasReferenceInputs: len(tx.ReferenceInputs()) > 0,
	}

	w := tx.Witnesses()
	if w != nil {
		switch {
		case len(w.PlutusV3Scripts()) > 0:
			info.ScriptType = "plutus_v3"
		case len(w.PlutusV2Scripts()) > 0:
			info.ScriptType = "plutus_v2"
		case len(w.PlutusV1Scripts()) > 0:
			info.ScriptType = "plutus_v1"
		case len(w.NativeScripts()) > 0:
			info.ScriptType = "native"
		default:
			info.ScriptType = "none"
		}
	} else {
		info.ScriptType = "none"
	}

	return info, nil
}

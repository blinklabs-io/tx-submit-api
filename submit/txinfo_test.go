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
	"bytes"
	"encoding/hex"
	"testing"

	gocbor "github.com/blinklabs-io/gouroboros/cbor"
)

const (
	// Conway tx with Plutus V3 script (witness key 7), minting (body key 9),
	// and reference inputs (body key 18).
	plutusV3MintRefTxHex = "84a900818258200000000000000000000000000000000000000000000000000000000000000000000183a300581d6000000000000000000000000000000000000000000000000000000000011a000f42400282005820923918e403bf43c34b4ef6b48eb2ee04babed17320d8d1b9ff9ad086e86f44eca200583900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001821a000f4240a2581c12593b4cbf7fdfd8636db99fe356437cd6af8539aadaa0a401964874a14474756e611b00005af3107a4000581c0c8eaf490c53afbf27e3d84a3b57da51fbafe5aa78443fcec2dc262ea14561696b656e182aa300583910000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001821a000f4240a1581c0c8eaf490c53afbf27e3d84a3b57da51fbafe5aa78443fcec2dc262ea14763617264616e6f0103d8184782034463666f6f02182a09a2581c12593b4cbf7fdfd8636db99fe356437cd6af8539aadaa0a401964874a14474756e611b00005af3107a4000581c0c8eaf490c53afbf27e3d84a3b57da51fbafe5aa78443fcec2dc262ea24763617264616e6f014561696b656e2d0b5820ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0d818258200000000000000000000000000000000000000000000000000000000000000000001082581d60000000000000000000000000000000000000000000000000000000001a3b9aca0011011281825820000000000000000000000000000000000000000000000000000000000000000000a30582840100d87980821a000f42401a05f5e100840101182a821a000f42401a05f5e1000481d879800782587d587b0101003232323232323225333333008001153330033370e900018029baa001153330073006375400224a66600894452615330054911856616c696461746f722072657475726e65642066616c73650013656002002002002002002153300249010b5f746d70323a20566f696400165734ae7155ceaab9e5573eae91589558930101003232323232323225333333008001153330033370e900018029baa001153330073006375400224a666008a6600a9201105f5f5f5f5f6d696e745f325f5f5f5f5f0014a22930a99802a4811856616c696461746f722072657475726e65642066616c73650013656002002002002002002153300249010b5f746d70323a20566f696400165734ae7155ceaab9e5573eae91f5f6"
)

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex decode failed: %v", err)
	}
	return b
}

// buildMinimalConwayBody returns a raw CBOR map for a Conway transaction body
// with one zero-hash input, one lovelace-only output to a zero-keyhash
// enterprise address (0x60 header), and a small fee.
//
// These transactions are structurally valid CBOR but would be rejected by a
// real cardano-node because they lack:
//   - Real UTXO inputs (zero-hash does not exist on chain)
//   - Vkey witnesses / signatures
//   - Correct fee calculation
//   - Script data hash (required when Plutus scripts are present)
//
// They exist solely to exercise ParseTxInfo, which only inspects CBOR
// structure and does not validate cryptographic or economic correctness.
func buildMinimalConwayBody() map[uint]any {
	// Enterprise address: 0x60 (type 6, mainnet) + 28-byte zero payment keyhash.
	addr := append([]byte{0x60}, make([]byte, 28)...)
	return map[uint]any{
		0: [][]any{{make([]byte, 32), uint32(0)}},        // inputs
		1: []map[uint]any{{0: addr, 1: uint64(1_000_000_000)}}, // outputs
		2: uint64(100_000),                               // fee
	}
}

// buildConwayTx encodes [body, witnesses, true, null] as a 4-element CBOR array.
func buildConwayTx(t *testing.T, body, witnesses any) []byte {
	t.Helper()
	bodyBytes, err := gocbor.Encode(body)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	witnessBytes, err := gocbor.Encode(witnesses)
	if err != nil {
		t.Fatalf("encode witnesses: %v", err)
	}
	txBytes, err := gocbor.Encode([]any{
		gocbor.RawMessage(bodyBytes),
		gocbor.RawMessage(witnessBytes),
		true,
		nil,
	})
	if err != nil {
		t.Fatalf("encode tx: %v", err)
	}
	return txBytes
}

// buildNativeScriptTx builds a minimal Conway transaction whose witness set
// contains one pubkey native script (key 1). No minting, no reference inputs.
func buildNativeScriptTx(t *testing.T) []byte {
	t.Helper()
	// Native pubkey script: [0, <28 bytes of 0x11>]
	nativeScript := []any{uint(0), bytes.Repeat([]byte{0x11}, 28)}
	witnesses := map[uint]any{1: []any{nativeScript}}
	return buildConwayTx(t, buildMinimalConwayBody(), witnesses)
}

// buildPlutusV1Tx builds a minimal Conway transaction whose witness set
// contains one Plutus V1 script (key 3). No minting, no reference inputs.
func buildPlutusV1Tx(t *testing.T) []byte {
	t.Helper()
	scriptBytes := mustDecodeHex(t, "510101003222253330044a229309b2b2b9a1")
	witnesses := map[uint]any{3: []any{scriptBytes}}
	return buildConwayTx(t, buildMinimalConwayBody(), witnesses)
}

// buildPlutusV2Tx builds a minimal Conway transaction whose witness set
// contains one Plutus V2 script (key 6). No minting, no reference inputs.
func buildPlutusV2Tx(t *testing.T) []byte {
	t.Helper()
	scriptBytes := mustDecodeHex(t, "510101003222253330044a229309b2b2b9a1")
	witnesses := map[uint]any{6: []any{scriptBytes}}
	return buildConwayTx(t, buildMinimalConwayBody(), witnesses)
}

func TestParseTxInfo_PlutusV1Script(t *testing.T) {
	t.Parallel()
	info, err := ParseTxInfo(buildPlutusV1Tx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ScriptType != "plutus_v1" {
		t.Errorf("ScriptType: want %q, got %q", "plutus_v1", info.ScriptType)
	}
	if info.HasMinting {
		t.Error("HasMinting: want false, got true")
	}
	if info.HasReferenceInputs {
		t.Error("HasReferenceInputs: want false, got true")
	}
}

func TestParseTxInfo_PlutusV3WithMintingAndRefInputs(t *testing.T) {
	t.Parallel()
	info, err := ParseTxInfo(mustDecodeHex(t, plutusV3MintRefTxHex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ScriptType != "plutus_v3" {
		t.Errorf("ScriptType: want %q, got %q", "plutus_v3", info.ScriptType)
	}
	if !info.HasMinting {
		t.Error("HasMinting: want true, got false")
	}
	if !info.HasReferenceInputs {
		t.Error("HasReferenceInputs: want true, got false")
	}
}

func TestParseTxInfo_NativeScript(t *testing.T) {
	t.Parallel()
	info, err := ParseTxInfo(buildNativeScriptTx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ScriptType != "native" {
		t.Errorf("ScriptType: want %q, got %q", "native", info.ScriptType)
	}
	if info.HasMinting {
		t.Error("HasMinting: want false, got true")
	}
	if info.HasReferenceInputs {
		t.Error("HasReferenceInputs: want false, got true")
	}
}

func TestParseTxInfo_PlutusV2Script(t *testing.T) {
	t.Parallel()
	info, err := ParseTxInfo(buildPlutusV2Tx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ScriptType != "plutus_v2" {
		t.Errorf("ScriptType: want %q, got %q", "plutus_v2", info.ScriptType)
	}
	if info.HasMinting {
		t.Error("HasMinting: want false, got true")
	}
	if info.HasReferenceInputs {
		t.Error("HasReferenceInputs: want false, got true")
	}
}

func TestParseTxInfo_InvalidCBOR(t *testing.T) {
	t.Parallel()
	_, err := ParseTxInfo([]byte("not-valid-cbor"))
	if err == nil {
		t.Error("expected error for invalid CBOR, got nil")
	}
}

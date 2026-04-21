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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func setup() {
	// Re-initialise vars for each test so counters start at zero.
	initVars()
}

func TestRecordTxRequest_Accepted(t *testing.T) {
	setup()
	RecordTxRequest("accepted")
	if got := testutil.ToFloat64(txSubmitRequestsTotal.WithLabelValues("accepted")); got != 1 {
		t.Errorf("expected 1, got %f", got)
	}
}

func TestRecordTxRequest_Rejected(t *testing.T) {
	setup()
	RecordTxRequest("rejected")
	if got := testutil.ToFloat64(txSubmitRequestsTotal.WithLabelValues("rejected")); got != 1 {
		t.Errorf("expected 1, got %f", got)
	}
}

func TestRecordTxRequest_Error(t *testing.T) {
	setup()
	RecordTxRequest("error")
	if got := testutil.ToFloat64(txSubmitRequestsTotal.WithLabelValues("error")); got != 1 {
		t.Errorf("expected 1, got %f", got)
	}
}

func TestRecordTxContent_NoScripts(t *testing.T) {
	setup()
	RecordTxContent("none", false, false)

	if got := testutil.ToFloat64(txSubmitScriptTypeTotal.WithLabelValues("none")); got != 1 {
		t.Errorf("script_type: expected 1, got %f", got)
	}
	if got := testutil.ToFloat64(txSubmitHasMintingTotal.WithLabelValues("false")); got != 1 {
		t.Errorf("has_minting: expected 1, got %f", got)
	}
	if got := testutil.ToFloat64(txSubmitHasReferenceInputsTotal.WithLabelValues("false")); got != 1 {
		t.Errorf("has_reference_inputs: expected 1, got %f", got)
	}
}

func TestRecordTxContent_PlutusV3WithMintingAndRefInputs(t *testing.T) {
	setup()
	RecordTxContent("plutus_v3", true, true)

	if got := testutil.ToFloat64(txSubmitScriptTypeTotal.WithLabelValues("plutus_v3")); got != 1 {
		t.Errorf("script_type: expected 1, got %f", got)
	}
	if got := testutil.ToFloat64(txSubmitHasMintingTotal.WithLabelValues("true")); got != 1 {
		t.Errorf("has_minting: expected 1, got %f", got)
	}
	if got := testutil.ToFloat64(txSubmitHasReferenceInputsTotal.WithLabelValues("true")); got != 1 {
		t.Errorf("has_reference_inputs: expected 1, got %f", got)
	}
}

func TestIncTxSubmitCount(t *testing.T) {
	setup()
	IncTxSubmitCount()
	if got := testutil.ToFloat64(txSubmitCount); got != 1 {
		t.Errorf("expected 1, got %f", got)
	}
}

func TestIncTxSubmitFailCount(t *testing.T) {
	setup()
	IncTxSubmitFailCount()
	if got := testutil.ToFloat64(txSubmitFailCount); got != 1 {
		t.Errorf("expected 1, got %f", got)
	}
}

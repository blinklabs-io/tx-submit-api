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
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Gauges kept to match input-output-hk's metric type.
	txSubmitFailCount prometheus.Gauge
	txSubmitCount     prometheus.Gauge

	// New counters — correct Prometheus type for monotonically increasing event counts.
	txSubmitRequestsTotal           *prometheus.CounterVec
	txSubmitScriptTypeTotal         *prometheus.CounterVec
	txSubmitHasMintingTotal         *prometheus.CounterVec
	txSubmitHasReferenceInputsTotal *prometheus.CounterVec

	registerOnce sync.Once
)

func init() {
	initVars()
}

func initVars() {
	txSubmitFailCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tx_submit_fail_count",
		Help: "transactions failed",
	})
	txSubmitCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tx_submit_count",
		Help: "transactions submitted",
	})
	txSubmitRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tx_submit_requests_total",
			Help: "Total transaction submission requests by result.",
		},
		[]string{"result"},
	)
	txSubmitScriptTypeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tx_submit_script_type_total",
			Help: "Transaction submissions by script type present in witness set.",
		},
		[]string{"type"},
	)
	txSubmitHasMintingTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tx_submit_has_minting_total",
			Help: "Transaction submissions by minting or burning presence.",
		},
		[]string{"has_minting"},
	)
	txSubmitHasReferenceInputsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tx_submit_has_reference_inputs_total",
			Help: "Transaction submissions by reference input presence.",
		},
		[]string{"has_reference_inputs"},
	)
}

// Register registers all collectors with the default Prometheus registry.
// Safe to call multiple times; registration happens exactly once.
func Register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			txSubmitFailCount,
			txSubmitCount,
			txSubmitRequestsTotal,
			txSubmitScriptTypeTotal,
			txSubmitHasMintingTotal,
			txSubmitHasReferenceInputsTotal,
		)
	})
}

// RegisterForTesting is a no-op kept for backwards compatibility with tests.
// Collectors are initialised in init() so they are always ready to record.
func RegisterForTesting() {}

func IncTxSubmitCount() {
	if txSubmitCount != nil {
		txSubmitCount.Inc()
	}
}

func IncTxSubmitFailCount() {
	if txSubmitFailCount != nil {
		txSubmitFailCount.Inc()
	}
}

// RecordTxRequest records a submission attempt. result is one of "accepted",
// "rejected" (node rejected the tx), or "error".
func RecordTxRequest(result string) {
	txSubmitRequestsTotal.WithLabelValues(result).Inc()
}

// RecordTxContent records content signals for a successfully parsed transaction.
// Call after ParseTxInfo succeeds, regardless of whether the node accepted the tx.
func RecordTxContent(scriptType string, hasMinting, hasReferenceInputs bool) {
	txSubmitScriptTypeTotal.WithLabelValues(scriptType).Inc()
	txSubmitHasMintingTotal.WithLabelValues(strconv.FormatBool(hasMinting)).Inc()
	txSubmitHasReferenceInputsTotal.WithLabelValues(strconv.FormatBool(hasReferenceInputs)).Inc()
}

// Getters used by tests in other packages.

func TxSubmitRequestsTotal() *prometheus.CounterVec {
	return txSubmitRequestsTotal
}

func TxSubmitScriptTypeTotal() *prometheus.CounterVec {
	return txSubmitScriptTypeTotal
}

func TxSubmitHasMintingTotal() *prometheus.CounterVec {
	return txSubmitHasMintingTotal
}

func TxSubmitHasReferenceInputsTotal() *prometheus.CounterVec {
	return txSubmitHasReferenceInputsTotal
}

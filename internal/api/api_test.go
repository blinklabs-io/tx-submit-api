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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/blinklabs-io/tx-submit-api/internal/config"
	"github.com/blinklabs-io/tx-submit-api/internal/logging"
	"github.com/blinklabs-io/tx-submit-api/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMain(m *testing.M) {
	logging.Setup(&config.LoggingConfig{Level: "error"})
	metrics.RegisterForTesting()
	os.Exit(m.Run())
}

// newTestMux delegates to newMux to ensure test routes always match production routes.
// Each call gets a fresh nodeHealthState so parallel tests don't share state.
func newTestMux(nh *nodeHealthState) http.Handler {
	mux := newMux(fstest.MapFS{}, nh)
	var handler http.Handler = mux
	handler = loggingMiddleware(nil)(handler)
	handler = recoveryMiddleware(handler)
	handler = corsMiddleware(handler)
	return handler
}

// --- realClientIP ---

func TestRealClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		xRealIP        string
		xForwarded     string
		remoteAddr     string
		trustedProxies []string
		want           string
	}{
		{
			name:           "X-Real-IP wins when peer is trusted",
			xRealIP:        "1.2.3.4",
			xForwarded:     "5.6.7.8",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{"9.10.11.12"},
			want:           "1.2.3.4",
		},
		{
			name:           "X-Forwarded-For single IP when peer is trusted",
			xForwarded:     "10.0.0.1",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{"9.10.11.12"},
			want:           "10.0.0.1",
		},
		{
			name:           "X-Forwarded-For multiple IPs returns first when peer is trusted",
			xForwarded:     "10.0.0.1, 172.16.0.1, 192.168.1.1",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{"9.10.11.12"},
			want:           "10.0.0.1",
		},
		{
			name:           "forwarded headers ignored when peer is not trusted",
			xRealIP:        "1.2.3.4",
			xForwarded:     "5.6.7.8",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{},
			want:           "9.10.11.12",
		},
		{
			name:           "trusted proxy matched by CIDR",
			xRealIP:        "1.2.3.4",
			remoteAddr:     "10.0.0.5:9000",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "1.2.3.4",
		},
		{
			name:       "RemoteAddr with port strips port",
			remoteAddr: "203.0.113.5:54321",
			want:       "203.0.113.5",
		},
		{
			name:       "RemoteAddr without port returned as-is",
			remoteAddr: "203.0.113.5",
			want:       "203.0.113.5",
		},
		{
			name:       "IPv6 RemoteAddr strips port",
			remoteAddr: "[::1]:8080",
			want:       "::1",
		},
		{
			name:           "invalid X-Real-IP falls back to RemoteAddr",
			xRealIP:        "not-an-ip",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{"9.10.11.12"},
			want:           "9.10.11.12",
		},
		{
			name:           "invalid X-Forwarded-For falls back to RemoteAddr",
			xForwarded:     "hostname.example.com",
			remoteAddr:     "9.10.11.12:9000",
			trustedProxies: []string{"9.10.11.12"},
			want:           "9.10.11.12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.xForwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwarded)
			}
			if got := realClientIP(req, tt.trustedProxies); got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

// --- liveness ---

func TestLiveness_OK(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newTestMux(&nodeHealthState{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse body: %s", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

// --- readiness ---

func TestReadiness_OK(t *testing.T) {
	t.Parallel()
	nh := &nodeHealthState{healthy: true}
	rec := httptest.NewRecorder()
	newTestMux(nh).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse body: %s", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

func TestReadiness_Unavailable(t *testing.T) {
	t.Parallel()
	nh := &nodeHealthState{healthy: false}
	rec := httptest.NewRecorder()
	newTestMux(nh).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse body: %s", err)
	}
	if body["status"] != "unavailable" {
		t.Errorf("expected status=unavailable, got %q", body["status"])
	}
}

// --- submit tx ---

func TestSubmitTx_ContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "missing content-type",
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "wrong content-type",
			contentType: "application/json",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/submit/tx", strings.NewReader("data"))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestSubmitTx_InvalidTxBytes(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/submit/tx", strings.NewReader("not-valid-cbor"))
	req.Header.Set("Content-Type", "application/cbor")
	newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

	// Invalid bytes fail transaction type detection before any node connection → 400
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- has tx ---

func TestHasTx_NoNode(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hastx/abc123", nil)
	newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

	// No node available → 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- CORS ---

func TestCORS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		method         string
		path           string
		wantStatus     int
		checkPreflight bool
	}{
		{
			name:           "preflight returns 204 with CORS headers",
			method:         http.MethodOptions,
			path:           "/api/submit/tx",
			wantStatus:     http.StatusNoContent,
			checkPreflight: true,
		},
		{
			name:       "normal response includes CORS origin header",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
			if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "*" {
				t.Errorf("expected Access-Control-Allow-Origin=*, got %q", v)
			}
			if tt.checkPreflight {
				for _, h := range []string{"Content-Type", "Accept"} {
					if v := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(v, h) {
						t.Errorf("expected Access-Control-Allow-Headers to contain %q, got %q", h, v)
					}
				}
			}
		})
	}
}

// --- metrics ---

func TestSubmitTx_RequestsTotal_InvalidCBOR(t *testing.T) {
	// Not parallel: reads counter value which is package-global state.
	// httptest.NewRequest sets RemoteAddr = "192.0.2.1:1234".
	const clientIP = "192.0.2.1"
	before := testutil.ToFloat64(metrics.TxSubmitRequestsTotal().WithLabelValues("error"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/submit/tx", strings.NewReader("not-valid-cbor"))
	req.Header.Set("Content-Type", "application/cbor")
	newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	after := testutil.ToFloat64(metrics.TxSubmitRequestsTotal().WithLabelValues("error"))
	if after-before != 1 {
		t.Errorf("requests_total{ip=%s,result=error}: expected increment of 1, got %f", clientIP, after-before)
	}
}

func TestSubmitTx_RequestsTotal_NoIncrementOnBadContentType(t *testing.T) {
	// Not parallel: reads counter value which is package-global state.
	const clientIP = "192.0.2.1"
	before := testutil.ToFloat64(metrics.TxSubmitRequestsTotal().WithLabelValues("error"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/submit/tx", strings.NewReader("data"))
	req.Header.Set("Content-Type", "application/json")
	newTestMux(&nodeHealthState{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
	after := testutil.ToFloat64(metrics.TxSubmitRequestsTotal().WithLabelValues("error"))
	if after != before {
		t.Errorf("requests_total should not increment on content-type rejection, got increment of %f", after-before)
	}
}

// --- health poller ---

func TestStartNodeHealthPoller_ZeroInterval(t *testing.T) {
	cfg := &config.Config{}
	cfg.Node.HealthCheckInterval = 0
	cfg.Node.Timeout = 1

	// Ensure startNodeHealthPoller does not panic when HealthCheckInterval is 0
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("startNodeHealthPoller panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startNodeHealthPoller(ctx, cfg)
}

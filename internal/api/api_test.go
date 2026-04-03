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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/blinklabs-io/tx-submit-api/internal/config"
	"github.com/blinklabs-io/tx-submit-api/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Setup(&config.LoggingConfig{Level: "error"})
	os.Exit(m.Run())
}

// newTestMux delegates to newMux to ensure test routes always match production routes.
func newTestMux() http.Handler {
	mux := newMux(fstest.MapFS{})
	var handler http.Handler = mux
	handler = loggingMiddleware(nil)(handler)
	handler = recoveryMiddleware(handler)
	handler = corsMiddleware(handler)
	return handler
}

// --- healthcheck ---

func TestHealthcheck_OK(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newTestMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthcheck", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse body: %s", err)
	}
	v, ok := body["failed"]
	if !ok {
		t.Fatal("expected key \"failed\" in response body")
	}
	if v != false {
		t.Errorf("expected failed=false, got %v", v)
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
			newTestMux().ServeHTTP(rec, req)

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
	newTestMux().ServeHTTP(rec, req)

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
	newTestMux().ServeHTTP(rec, req)

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
			path:       "/healthcheck",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			newTestMux().ServeHTTP(rec, req)

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

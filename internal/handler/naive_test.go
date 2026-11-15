package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var samplePayload []byte

func init() {
	var err error
	samplePayload, err = os.ReadFile("../../scripts/payload_sample.json")
	if err != nil {
		// Fallback embedded if path relative to test root
		samplePayload = []byte(`{"event_id":"evt_test_123","tenant_id":"tenant_test","user_id":123,"timestamp":1731650400,"action":"checkout","metrics":{"cpu_usage":10.5,"memory_bytes":1024,"duration_ms":5.0,"error_count":0,"throughput_qps":100},"tags":["prod","test"],"metadata":{"env":"test"}}`)
	}
}

func TestNaiveHandler_ServeHTTP_Success(t *testing.T) {
	h := NewNaiveHandler("node-test-01")

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(samplePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestNaiveHandler_ServeHTTP_InvalidMethod(t *testing.T) {
	h := NewNaiveHandler("node-test-01")
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func BenchmarkNaiveHandler_ServeHTTP(b *testing.B) {
	h := NewNaiveHandler("node-bench-01")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(samplePayload))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Benchmark request failed with status %d", w.Code)
		}
	}
}

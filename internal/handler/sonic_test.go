package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSonicHandler_ServeHTTP_Success(t *testing.T) {
	h := NewSonicHandler("node-test-01")

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

func BenchmarkSonicHandler_ServeHTTP(b *testing.B) {
	h := NewSonicHandler("node-bench-01")

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

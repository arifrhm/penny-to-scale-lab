package handler

import (
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestRingBufferHandler_HandleFastHTTP_Success(t *testing.T) {
	h := NewRingBufferHandler("node-ring-01", 2)
	defer h.Close()

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody(samplePayload)

	h.HandleFastHTTP(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}

	contentType := string(ctx.Response.Header.ContentType())
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Give consumers a small window to drain
	time.Sleep(10 * time.Millisecond)
	if h.ConsumedCount() == 0 {
		t.Errorf("Expected at least 1 consumed event, got %d", h.ConsumedCount())
	}
}

func BenchmarkRingBufferHandler_Serve(b *testing.B) {
	h := NewRingBufferHandler("node-ring-bench", 4)
	defer h.Close()

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody(samplePayload)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h.HandleFastHTTP(&ctx)
		ctx.Response.Reset()
	}
}

package handler

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestZeroAllocHandler_HandleFastHTTP_Success(t *testing.T) {
	h := NewZeroAllocHandler("node-test-01")

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
}

func BenchmarkZeroAllocHandler_Serve(b *testing.B) {
	h := NewZeroAllocHandler("node-bench-01")

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

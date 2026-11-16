package handler

import (
	"bytes"
	"encoding/hex"
	"hash"
	"strconv"
	"time"

	"github.com/arifrhm/penny-to-scale-lab/internal/models"
	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"
)

// FastHTTPHandler represents V0.4.0 high-performance engine optimization.
// It leverages valyala/fasthttp's zero-copy request buffer reading (ctx.PostBody()),
// internal worker pool connection recycling, and ByteDance Sonic JIT/SIMD parsing
// to eliminate standard library net/http allocation boundaries.
type FastHTTPHandler struct {
	WorkerID      string
	workerIDBytes []byte
}

func NewFastHTTPHandler(workerID string) *FastHTTPHandler {
	return &FastHTTPHandler{
		WorkerID:      workerID,
		workerIDBytes: []byte(workerID),
	}
}

func (h *FastHTTPHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		ctx.SetBodyString("Method not allowed")
		return
	}

	start := time.Now()

	// 1. Zero-copy request body access directly from fasthttp connection buffer
	bodyBytes := ctx.PostBody()

	// 2. Recycled struct & Sonic JIT SIMD Unmarshal
	req := eventRequestPool.Get().(*models.EventRequest)
	req.Tags = req.Tags[:0]
	clear(req.Metadata)
	defer eventRequestPool.Put(req)

	if err := sonic.ConfigFastest.Unmarshal(bodyBytes, req); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("Malformed JSON payload")
		return
	}

	// 3. Validation
	if err := req.Validate(); err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		ctx.SetBodyString(err.Error())
		return
	}

	// 4. Recycled Hasher for Checksum
	hasher := hasherPool.Get().(hash.Hash)
	hasher.Reset()
	defer hasherPool.Put(hasher)

	hasher.Write([]byte(req.EventID))
	hasher.Write([]byte(req.TenantID))
	checksumBytes := hasher.Sum(nil)
	checksum := hex.EncodeToString(checksumBytes)

	// Audit Hash without fmt.Sprintf
	hasher.Reset()
	hasher.Write([]byte(req.EventID))
	hasher.Write([]byte{':'})
	hasher.Write([]byte(req.TenantID))
	hasher.Write([]byte{':'})
	var scratch [32]byte
	hasher.Write(strconv.AppendInt(scratch[:0], req.Timestamp, 10))
	hasher.Write([]byte{':'})
	hasher.Write(h.workerIDBytes)
	auditBytes := hasher.Sum(nil)
	auditHash := hex.EncodeToString(auditBytes)

	resp := models.EventResponse{
		Status:      "SUCCESS",
		EventID:     req.EventID,
		TenantID:    req.TenantID,
		Checksum:    checksum,
		ProcessedAt: start.UnixNano(),
		WorkerNode:  h.WorkerID,
		TagsCount:   len(req.Tags),
		AuditHash:   auditHash,
	}

	// 5. Sonic JIT SIMD Response Serialization directly into connection body
	respBuf := bufferPool.Get().(*bytes.Buffer)
	respBuf.Reset()
	defer bufferPool.Put(respBuf)

	encoder := sonic.ConfigFastest.NewEncoder(respBuf)
	if err := encoder.Encode(&resp); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString("Failed to serialize response")
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(respBuf.Bytes())
}

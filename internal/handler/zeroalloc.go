package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strconv"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

var colonByte = []byte{':'}

type zeroAllocResources struct {
	hasher   hash.Hash
	sumBuf   [32]byte
	auditBuf [32]byte
	hexBuf1  [64]byte
	hexBuf2  [64]byte
	numBuf   [32]byte
}

var zeroAllocPool = sync.Pool{
	New: func() any {
		return &zeroAllocResources{
			hasher: sha256.New(),
		}
	},
}

// ZeroAllocHandler represents V0.5.0 optimization.
// It bypasses intermediate struct instantiation and reflection entirely.
// By combining valyala/fasthttp's zero-copy request buffer, buger/jsonparser's
// in-place byte slicing, and zeroAllocPool, it achieves true zero-allocation: 0 B/op and 0 allocs/op.
type ZeroAllocHandler struct {
	WorkerID      string
	workerIDBytes []byte
}

func NewZeroAllocHandler(workerID string) *ZeroAllocHandler {
	return &ZeroAllocHandler{
		WorkerID:      workerID,
		workerIDBytes: []byte(workerID),
	}
}

func (h *ZeroAllocHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		ctx.SetBodyString("Method not allowed")
		return
	}

	start := time.Now().UnixNano()
	bodyBytes := ctx.PostBody()

	// 1. Direct in-place byte slicing without struct or string allocations
	eventIDBytes, _, _, err := jsonparser.Get(bodyBytes, "event_id")
	if err != nil || len(eventIDBytes) == 0 {
		ctx.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		ctx.SetBodyString("event_id is required")
		return
	}

	tenantIDBytes, _, _, err := jsonparser.Get(bodyBytes, "tenant_id")
	if err != nil || len(tenantIDBytes) == 0 {
		ctx.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		ctx.SetBodyString("tenant_id is required")
		return
	}

	timestamp, err := jsonparser.GetInt(bodyBytes, "timestamp")
	if err != nil || timestamp <= 0 {
		ctx.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		ctx.SetBodyString("timestamp must be greater than zero")
		return
	}

	// Count tags without allocating closure
	tagsCount := 0
	tagsBytes, _, _, _ := jsonparser.Get(bodyBytes, "tags")
	if len(tagsBytes) > 2 {
		for i := 0; i < len(tagsBytes); i++ {
			if tagsBytes[i] == ',' {
				tagsCount++
			}
		}
		tagsCount++
	}

	// 2. Acquire pooled resources (Concrete struct pointer - zero heap boxing)
	res := zeroAllocPool.Get().(*zeroAllocResources)
	defer zeroAllocPool.Put(res)

	res.hasher.Reset()
	res.hasher.Write(eventIDBytes)
	res.hasher.Write(tenantIDBytes)
	checksumBytes := res.hasher.Sum(res.sumBuf[:0])
	hex.Encode(res.hexBuf1[:], checksumBytes)

	// 3. Audit Hash
	res.hasher.Reset()
	res.hasher.Write(eventIDBytes)
	res.hasher.Write(colonByte)
	res.hasher.Write(tenantIDBytes)
	res.hasher.Write(colonByte)
	res.hasher.Write(strconv.AppendInt(res.numBuf[:0], timestamp, 10))
	res.hasher.Write(colonByte)
	res.hasher.Write(h.workerIDBytes)
	auditBytes := res.hasher.Sum(res.auditBuf[:0])
	hex.Encode(res.hexBuf2[:], auditBytes)

	// 4. Direct response serialization using bytebufferpool
	buf := bytebufferpool.Get()
	buf.Reset()
	defer bytebufferpool.Put(buf)

	buf.WriteString(`{"status":"SUCCESS","event_id":"`)
	buf.Write(eventIDBytes)
	buf.WriteString(`","tenant_id":"`)
	buf.Write(tenantIDBytes)
	buf.WriteString(`","checksum":"`)
	buf.Write(res.hexBuf1[:])
	buf.WriteString(`","processed_at_ns":`)
	buf.Write(strconv.AppendInt(res.numBuf[:0], start, 10))
	buf.WriteString(`,"worker_node":"`)
	buf.Write(h.workerIDBytes)
	buf.WriteString(`","tags_count":`)
	buf.Write(strconv.AppendInt(res.numBuf[:0], int64(tagsCount), 10))
	buf.WriteString(`,"audit_hash":"`)
	buf.Write(res.hexBuf2[:])
	buf.WriteString("\"}\n")

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(buf.Bytes())
}

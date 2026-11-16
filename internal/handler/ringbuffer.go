package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/buger/jsonparser"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

const (
	// RingBufferSize must be a power of 2 for fast bitwise masking.
	RingBufferSize = 1 << 16 // 65,536 slots (~16MB pre-allocated)
	RingBufferMask = RingBufferSize - 1
)

type EventSlot struct {
	EventID     [64]byte
	EventIDLen  int
	TenantID    [32]byte
	TenantIDLen int
	Timestamp   int64
	IngestNs    int64
	Sequence    uint64
	Ready       uint32 // 1 when written, 0 when consumed
}

type RingBufferResources struct {
	hasher   hash.Hash
	sumBuf   [32]byte
	auditBuf [32]byte
	hexBuf   [64]byte
	numBuf   [32]byte
}

var ringResourcesPool = sync.Pool{
	New: func() any {
		return &RingBufferResources{
			hasher: sha256.New(),
		}
	},
}

// RingBufferHandler represents V0.6.0 Lock-Free Ring Buffer Ingestion (LMAX Disruptor Pattern).
// HTTP handlers write incoming events directly into a lock-free circular ring buffer with atomic CAS.
// Dedicated background batch consumers drain the buffer and perform audit hashing asynchronously,
// decoupling ingestion throughput and tail latency from downstream computation.
type RingBufferHandler struct {
	WorkerID      string
	workerIDBytes []byte
	slots         []EventSlot
	writeCursor   uint64
	readCursor    uint64
	consumedCount uint64
	droppedCount  uint64
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

func NewRingBufferHandler(workerID string, numConsumers int) *RingBufferHandler {
	if numConsumers <= 0 {
		numConsumers = runtime.NumCPU()
	}

	h := &RingBufferHandler{
		WorkerID:      workerID,
		workerIDBytes: []byte(workerID),
		slots:         make([]EventSlot, RingBufferSize),
		stopChan:      make(chan struct{}),
	}

	// Launch background batch worker consumers
	for i := 0; i < numConsumers; i++ {
		h.wg.Add(1)
		go h.consumerLoop()
	}

	return h
}

func (h *RingBufferHandler) Close() {
	close(h.stopChan)
	h.wg.Wait()
}

func (h *RingBufferHandler) ConsumedCount() uint64 {
	return atomic.LoadUint64(&h.consumedCount)
}

func (h *RingBufferHandler) DroppedCount() uint64 {
	return atomic.LoadUint64(&h.droppedCount)
}

func (h *RingBufferHandler) consumerLoop() {
	defer h.wg.Done()

	res := ringResourcesPool.Get().(*RingBufferResources)
	defer ringResourcesPool.Put(res)

	for {
		select {
		case <-h.stopChan:
			return
		default:
			// Claim next read slot atomically
			rSeq := atomic.LoadUint64(&h.readCursor)
			wSeq := atomic.LoadUint64(&h.writeCursor)

			if rSeq >= wSeq {
				runtime.Gosched()
				continue
			}

			if !atomic.CompareAndSwapUint64(&h.readCursor, rSeq, rSeq+1) {
				continue
			}

			slotIdx := rSeq & RingBufferMask
			slot := &h.slots[slotIdx]

			// Spin-wait until writer marks slot as Ready, checking stopChan to prevent deadlock
			for atomic.LoadUint32(&slot.Ready) == 0 {
				select {
				case <-h.stopChan:
					return
				default:
					runtime.Gosched()
				}
			}

			// Process audit hash asynchronously in background
			res.hasher.Reset()
			res.hasher.Write(slot.EventID[:slot.EventIDLen])
			res.hasher.Write(colonByte)
			res.hasher.Write(slot.TenantID[:slot.TenantIDLen])
			res.hasher.Write(colonByte)
			res.hasher.Write(strconv.AppendInt(res.numBuf[:0], slot.Timestamp, 10))
			res.hasher.Write(colonByte)
			res.hasher.Write(h.workerIDBytes)
			digest := res.hasher.Sum(res.auditBuf[:0])
			hex.Encode(res.hexBuf[:], digest)

			// Mark slot as consumed
			atomic.StoreUint32(&slot.Ready, 0)
			atomic.AddUint64(&h.consumedCount, 1)
		}
	}
}

func (h *RingBufferHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		ctx.SetBodyString("Method not allowed")
		return
	}

	start := time.Now().UnixNano()
	bodyBytes := ctx.PostBody()

	// 1. Direct in-place byte slicing without heap allocation
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

	// 2. Claim next sequence in lock-free ring buffer via CAS (with saturation protection)
	var seq uint64
	for {
		wSeq := atomic.LoadUint64(&h.writeCursor)
		rSeq := atomic.LoadUint64(&h.readCursor)
		if wSeq-rSeq >= RingBufferSize {
			atomic.AddUint64(&h.droppedCount, 1)
			ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
			ctx.SetBodyString("Ring buffer saturated (backpressure)")
			return
		}
		if atomic.CompareAndSwapUint64(&h.writeCursor, wSeq, wSeq+1) {
			seq = wSeq
			break
		}
	}

	slotIdx := seq & RingBufferMask
	slot := &h.slots[slotIdx]

	// 3. Write event fields into pre-allocated slot without heap allocation
	slot.EventIDLen = copy(slot.EventID[:], eventIDBytes)
	slot.TenantIDLen = copy(slot.TenantID[:], tenantIDBytes)
	slot.Timestamp = timestamp
	slot.IngestNs = start
	slot.Sequence = seq
	atomic.StoreUint32(&slot.Ready, 1)

	// 4. Ultra-fast serialization from pooled buffer (0 B/op)
	res := ringResourcesPool.Get().(*RingBufferResources)
	defer ringResourcesPool.Put(res)

	buf := bytebufferpool.Get()
	buf.Reset()
	defer bytebufferpool.Put(buf)

	buf.WriteString(`{"status":"QUEUED","event_id":"`)
	buf.Write(eventIDBytes)
	buf.WriteString(`","tenant_id":"`)
	buf.Write(tenantIDBytes)
	buf.WriteString(`","sequence":`)
	buf.Write(strconv.AppendUint(res.numBuf[:0], seq, 10))
	buf.WriteString(`,"ingest_ns":`)
	buf.Write(strconv.AppendInt(res.numBuf[:0], start, 10))
	buf.WriteString(`,"worker_node":"`)
	buf.Write(h.workerIDBytes)
	buf.WriteString("\"}\n")

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(buf.Bytes())
}

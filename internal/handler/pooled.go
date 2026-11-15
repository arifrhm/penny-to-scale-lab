package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/arifrhm/penny-to-scale-lab/internal/models"
)

// bufferPool recycles byte buffers to eliminate io.ReadAll and serialization heap churn.
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

// hasherPool recycles sha256.Hash instances to eliminate heap allocations per request.
var hasherPool = sync.Pool{
	New: func() interface{} {
		return sha256.New()
	},
}

// eventRequestPool recycles EventRequest structs and their internal map/slice capacities.
var eventRequestPool = sync.Pool{
	New: func() interface{} {
		return &models.EventRequest{
			Tags:     make([]string, 0, 16),
			Metadata: make(map[string]string, 16),
		}
	},
}

// PooledHandler represents V0.2.0 optimization.
// It recycles I/O buffers, hashers, and request structs using sync.Pool,
// eliminates fmt.Sprintf allocations via strconv byte-level appending,
// and significantly cuts down B/op, allocs/op, and GC pressure.
type PooledHandler struct {
	WorkerID      string
	workerIDBytes []byte
}

func NewPooledHandler(workerID string) *PooledHandler {
	return &PooledHandler{
		WorkerID:      workerID,
		workerIDBytes: []byte(workerID),
	}
}

func (h *PooledHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	// 1. Get recycled buffer for request body
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if _, err := buf.ReadFrom(r.Body); err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Recycled EventRequest struct
	req := eventRequestPool.Get().(*models.EventRequest)
	req.Tags = req.Tags[:0]
	clear(req.Metadata)
	defer eventRequestPool.Put(req)

	if err := json.Unmarshal(buf.Bytes(), req); err != nil {
		http.Error(w, "Malformed JSON payload", http.StatusBadRequest)
		return
	}

	// 3. Validation
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// 4. Recycled Hasher for Checksum (Zero heap alloc)
	hasher := hasherPool.Get().(hash.Hash)
	hasher.Reset()
	defer hasherPool.Put(hasher)

	hasher.Write([]byte(req.EventID))
	hasher.Write([]byte(req.TenantID))
	checksumBytes := hasher.Sum(nil)
	checksum := hex.EncodeToString(checksumBytes)

	// Audit Hash without fmt.Sprintf string allocation
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

	// 5. Serialize into recycled response buffer
	respBuf := bufferPool.Get().(*bytes.Buffer)
	respBuf.Reset()
	defer bufferPool.Put(respBuf)

	encoder := json.NewEncoder(respBuf)
	if err := encoder.Encode(&resp); err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBuf.Bytes())
}

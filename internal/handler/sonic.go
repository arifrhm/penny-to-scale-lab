package handler

import (
	"bytes"
	"encoding/hex"
	"hash"
	"net/http"
	"strconv"
	"time"

	"github.com/arifrhm/penny-to-scale-lab/internal/models"
	"github.com/bytedance/sonic"
)

// SonicHandler represents V0.3.0 optimization.
// It combines sync.Pool recycling with ByteDance's Sonic JIT/SIMD-accelerated
// zero-reflection JSON parser and serializer, slashing CPU instruction cycles and heap allocations.
type SonicHandler struct {
	WorkerID      string
	workerIDBytes []byte
}

func NewSonicHandler(workerID string) *SonicHandler {
	return &SonicHandler{
		WorkerID:      workerID,
		workerIDBytes: []byte(workerID),
	}
}

func (h *SonicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	// 1. Recycled buffer for incoming request body
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if _, err := buf.ReadFrom(r.Body); err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Recycled struct & Sonic JIT SIMD Unmarshal
	req := eventRequestPool.Get().(*models.EventRequest)
	req.Tags = req.Tags[:0]
	clear(req.Metadata)
	defer eventRequestPool.Put(req)

	if err := sonic.ConfigFastest.Unmarshal(buf.Bytes(), req); err != nil {
		http.Error(w, "Malformed JSON payload", http.StatusBadRequest)
		return
	}

	// 3. Validation
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
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

	// 5. Sonic JIT SIMD Response Serialization into recycled buffer
	respBuf := bufferPool.Get().(*bytes.Buffer)
	respBuf.Reset()
	defer bufferPool.Put(respBuf)

	encoder := sonic.ConfigFastest.NewEncoder(respBuf)
	if err := encoder.Encode(&resp); err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBuf.Bytes())
}

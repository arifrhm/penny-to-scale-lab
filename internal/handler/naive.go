package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/arifrhm/penny-to-scale-lab/internal/models"
)

// NaiveHandler represents the baseline V0.1.0 implementation.
// It relies on standard library net/http, full body buffer allocation via io.ReadAll,
// reflection-heavy encoding/json Unmarshal, and standard json.Marshal serialization.
type NaiveHandler struct {
	WorkerID string
}

func NewNaiveHandler(workerID string) *NaiveHandler {
	return &NaiveHandler{WorkerID: workerID}
}

func (h *NaiveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	// 1. Read entire body (allocates slice on heap each request)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Reflection-based unmarshaling
	var req models.EventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Malformed JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// 3. Validation
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// 4. Server-side transformation & checksum calculation
	hasher := sha256.New()
	hasher.Write([]byte(req.EventID))
	hasher.Write([]byte(req.TenantID))
	checksum := hex.EncodeToString(hasher.Sum(nil))

	auditHasher := sha256.New()
	auditHasher.Write([]byte(fmt.Sprintf("%s:%s:%d:%s", req.EventID, req.TenantID, req.Timestamp, h.WorkerID)))
	auditHash := hex.EncodeToString(auditHasher.Sum(nil))

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

	// 5. Reflection-based marshaling
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

package models

import (
	"errors"
	"strings"
)

var (
	ErrMissingEventID   = errors.New("event_id is required")
	ErrMissingTenantID  = errors.New("tenant_id is required")
	ErrInvalidTimestamp = errors.New("timestamp must be greater than zero")
)

type EventMetrics struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryBytes   int64   `json:"memory_bytes"`
	DurationMs    float64 `json:"duration_ms"`
	ErrorCount    int     `json:"error_count"`
	ThroughputQPS float64 `json:"throughput_qps"`
}

type EventRequest struct {
	EventID   string            `json:"event_id"`
	TenantID  string            `json:"tenant_id"`
	UserID    int64             `json:"user_id"`
	Timestamp int64             `json:"timestamp"`
	Action    string            `json:"action"`
	Metrics   EventMetrics      `json:"metrics"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
}

func (e *EventRequest) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(e.TenantID) == "" {
		return ErrMissingTenantID
	}
	if e.Timestamp <= 0 {
		return ErrInvalidTimestamp
	}
	return nil
}

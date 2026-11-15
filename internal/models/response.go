package models

type EventResponse struct {
	Status      string `json:"status"`
	EventID     string `json:"event_id"`
	TenantID    string `json:"tenant_id"`
	Checksum    string `json:"checksum"`
	ProcessedAt int64  `json:"processed_at_ns"`
	WorkerNode  string `json:"worker_node"`
	TagsCount   int    `json:"tags_count"`
	AuditHash   string `json:"audit_hash"`
}

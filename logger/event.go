package logger

// Exception represents a structured error payload for logs.
type Exception struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
	Stack   string `json:"stack,omitempty"`
}

// Event represents a structured log event with required and optional fields.
//
// Note: This struct is primarily used as an internal carrier for field values.
// The logger writes fields using zerolog; not all struct fields are serialized directly.
type Event struct {
	// Required / baseline fields
	Service        string `json:"service"`
	ServiceVersion string `json:"service_version,omitempty"`
	Environment    string `json:"environment,omitempty"`
	Component      string `json:"component,omitempty"`

	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id,omitempty"`

	// event is the canonical field name for business/saga event names.
	Event string `json:"event,omitempty"`

	// Location/context fields
	Function   string `json:"function,omitempty"`
	ErrorPath  string `json:"error_path,omitempty"`
	Topic      string `json:"topic,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	PayloadID  string `json:"payload_id,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`

	// Additional structured context
	Metadata  map[string]any `json:"metadata,omitempty"`
	Exception *Exception     `json:"exception,omitempty"`
}

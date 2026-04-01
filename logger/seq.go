package logger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// seqEventPayload represents the payload format expected by Seq's raw events API.
// EXTENSION POINT: Extend this struct to support more Seq features (properties, exception, etc.).
type seqEventPayload struct {
	Events []seqEvent `json:"Events"`
}

type seqEvent struct {
	Timestamp time.Time              `json:"Timestamp"`
	Level     string                 `json:"Level"`
	Message   string                 `json:"MessageTemplate"`
	Properties map[string]any        `json:"Properties,omitempty"`
}

// seqWriter implements zerolog.LevelWriter for sending events to Seq.
type seqWriter struct {
	client  *http.Client
	url     string
	apiKey  string
	service string
	redactKeys []string
}

// newSeqWriter constructs a new seqWriter based on global configuration.
// EXTENSION POINT: Customize HTTP client (timeouts, retries, proxies, etc.).
func newSeqWriter(cfg Config) zerolog.LevelWriter {
	if !cfg.EnableSeq || cfg.SeqURL == "" {
		return nil
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	return &seqWriter{
		client:  client,
		url:     cfg.SeqURL + "/api/events/raw",
		apiKey:  cfg.SeqAPIKey,
		service: cfg.Service,
		redactKeys: cfg.RedactKeys,
	}
}

// Write implements io.Writer to satisfy zerolog.LevelWriter interface expectations.
func (w *seqWriter) Write(p []byte) (int, error) {
	// Default to info level when level is not explicitly provided.
	return w.WriteLevel(zerolog.InfoLevel, p)
}

// WriteLevel implements zerolog.LevelWriter.
// Seq failures must never crash the app; all errors are swallowed.
func (w *seqWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}

	// Perform HTTP request asynchronously so logging does not block the main flow.
	go func() {
		var properties map[string]any
		_ = json.Unmarshal(p, &properties) // ignore error; fall back to empty map
		properties = redactMap(w.redactKeys, properties)

		payload := seqEventPayload{
			Events: []seqEvent{
				{
					Timestamp: time.Now().UTC(),
					Level:     level.String(),
					Message:   "{Message}", // generic message template
					Properties: properties,
				},
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return
		}

		req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/vnd.serilog.events+json")
		if w.apiKey != "" {
			req.Header.Set("X-Seq-ApiKey", w.apiKey)
		}

		// Fire-and-forget: ignore response and errors.
		resp, err := w.client.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()

	return len(p), nil
}


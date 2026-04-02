package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

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
		url:     cfg.SeqURL + "/ingest/clef",
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

	// zerolog reuses its buffer after WriteLevel returns; copy before entering goroutine.
	pCopy := bytes.Clone(p)
	// Perform HTTP request asynchronously so logging does not block the main flow.
	go func() {
		var event map[string]any
		_ = json.Unmarshal(pCopy, &event) // ignore error; fall back to empty map
		event = redactMap(w.redactKeys, event)

		// Remap zerolog fields to CLEF reserved fields.
		if ts, ok := event["time"].(string); ok {
			event["@t"] = ts
		} else {
			event["@t"] = time.Now().UTC().Format(time.RFC3339Nano)
		}
		delete(event, "time")

		if msg, ok := event["message"].(string); ok {
			event["@m"] = msg
		}
		delete(event, "message")

		event["@l"] = zeroToSeqLevel(level)
		delete(event, "level")

		if exc, ok := event["exception"].(map[string]any); ok {
			t, _ := exc["type"].(string)
			m, _ := exc["message"].(string)
			s, _ := exc["stack"].(string)
			event["@x"] = fmt.Sprintf("%s: %s\n%s", t, m, s)
			delete(event, "exception")
		}

		body, err := json.Marshal(event)
		if err != nil {
			return
		}

		req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/vnd.serilog.clef")
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

// zeroToSeqLevel maps zerolog levels to Serilog/Seq level names used in CLEF.
func zeroToSeqLevel(level zerolog.Level) string {
	switch level {
	case zerolog.TraceLevel:
		return "Verbose"
	case zerolog.DebugLevel:
		return "Debug"
	case zerolog.InfoLevel:
		return "Information"
	case zerolog.WarnLevel:
		return "Warning"
	case zerolog.ErrorLevel:
		return "Error"
	default:
		return "Fatal"
	}
}


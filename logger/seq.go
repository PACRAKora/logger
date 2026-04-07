package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

// seqWriter implements zerolog.LevelWriter, sending events to Seq via GELF over UDP.
type seqWriter struct {
	writer     *gelf.UDPWriter
	hostname   string
	service    string
	redactKeys []string
}

// newSeqWriter constructs a seqWriter using GELF UDP transport.
// SeqURL must be a UDP address (e.g. "localhost:12201").
// Returns nil when Seq is disabled or the address is empty.
func newSeqWriter(cfg Config) zerolog.LevelWriter {
	if !cfg.EnableSeq || cfg.SeqURL == "" {
		return nil
	}

	w, err := gelf.NewUDPWriter(cfg.SeqURL)
	if err != nil {
		return nil
	}

	hostname, _ := os.Hostname()

	return &seqWriter{
		writer:     w,
		hostname:   hostname,
		service:    cfg.Service,
		redactKeys: cfg.RedactKeys,
	}
}

// Write implements io.Writer; defaults to InfoLevel.
func (w *seqWriter) Write(p []byte) (int, error) {
	return w.WriteLevel(zerolog.InfoLevel, p)
}

// WriteLevel implements zerolog.LevelWriter.
// Seq failures are swallowed; the caller is never blocked.
func (w *seqWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}

	pCopy := bytes.Clone(p)
	go func() {
		var event map[string]any
		_ = json.Unmarshal(pCopy, &event)
		event = redactMap(w.redactKeys, event)

		// --- Short (required GELF field) ---
		short := ""
		if msg, ok := event["message"].(string); ok {
			short = msg
		} else {
			short = string(pCopy)
		}

		// --- TimeUnix ---
		var timeUnix float64
		if ts, ok := event["timestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				timeUnix = float64(t.UnixNano()) / 1e9
			}
		}
		if timeUnix == 0 {
			timeUnix = float64(time.Now().UnixNano()) / 1e9
		}

		// --- Full (exception → GELF Full field) ---
		full := ""
		if exc, ok := event["exception"].(map[string]any); ok {
			t, _ := exc["type"].(string)
			m, _ := exc["message"].(string)
			s, _ := exc["stack"].(string)
			full = fmt.Sprintf("%s: %s\n%s", t, m, s)
		}

		// --- Extra fields ---
		skip := map[string]bool{
			"message":   true,
			"timestamp": true,
			"level":     true,
			"exception": true,
		}
		extra := make(map[string]any)
		for k, v := range event {
			if skip[k] {
				continue
			}
			switch k {
			case "trace_id":
				extra["_TraceId"] = v
			case "span_id":
				extra["_SpanId"] = v
			case "parent_id":
				extra["_ParentId"] = v
			case "metadata":
				// Flatten metadata keys directly into extra.
				if m, ok := v.(map[string]any); ok {
					for mk, mv := range m {
						extra["_"+mk] = mv
					}
				}
			default:
				extra["_"+k] = v
			}
		}

		msg := &gelf.Message{
			Version:  "1.1",
			Host:     w.hostname,
			Short:    short,
			TimeUnix: timeUnix,
			Level:    zeroToGELFLevel(level),
			Extra:    extra,
		}
		if full != "" {
			msg.Full = full
		}

		_ = w.writer.WriteMessage(msg)
	}()

	return len(p), nil
}

// zeroToGELFLevel maps zerolog levels to syslog severity numbers used in GELF.
func zeroToGELFLevel(level zerolog.Level) int32 {
	switch level {
	case zerolog.TraceLevel, zerolog.DebugLevel:
		return 7 // Debug
	case zerolog.InfoLevel:
		return 6 // Informational
	case zerolog.WarnLevel:
		return 4 // Warning
	case zerolog.ErrorLevel:
		return 3 // Error
	default:
		return 2 // Critical
	}
}

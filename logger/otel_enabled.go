//go:build otel

package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// WithTraceFromContext extracts trace/span IDs from an OpenTelemetry span context (if present)
// and attaches them to the log event as `trace_id` and `span_id`.
func WithTraceFromContext(ctx context.Context) Option {
	return func(e *Event) {
		sc := trace.SpanContextFromContext(ctx)
		if !sc.IsValid() {
			return
		}
		e.TraceID = sc.TraceID().String()
		e.SpanID = sc.SpanID().String()
	}
}


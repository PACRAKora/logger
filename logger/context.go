package logger

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const (
	// CONFIGURABLE: Context key names can be changed if needed.
	ctxKeyTraceID ctxKey = "trace_id"
	ctxKeySpanID  ctxKey = "span_id"
)

// WithTraceID attaches the given trace ID to the context.
// REQUIRED FIELD: Trace ID is used to tie together related log events.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

// WithSpanID attaches the given span ID to the context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, ctxKeySpanID, spanID)
}

// TraceIDFromContext extracts the trace ID from the context.
// Returns "" if not found.
func TraceIDFromContext(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if v := ctx.Value(ctxKeyTraceID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return ctx, s
		}
	}
	return ctx, ""
}

// SpanIDFromContext extracts the span ID from the context.
// Returns "" if not found.
func SpanIDFromContext(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if v := ctx.Value(ctxKeySpanID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return ctx, s
		}
	}
	return ctx, ""
}
